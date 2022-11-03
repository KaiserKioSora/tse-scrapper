package urna

import (
	"fmt"
	"github.com/ahmetb/go-linq/v3"
	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"tse-scrapper/helpers"
	"tse-scrapper/modelos"
)

const (
	posfixoNomeCarga   = "%s-p000%s-cs.json"
	urlAuxModelo       = "https://resultados.tse.jus.br/oficial/ele2022/arquivo-urna/%s/config/%s/%s"
	pastaConfiguracoes = "configuracoes/"
)

func BaixarConfiguracaoDosEstados(params modelos.Parametros) error {
	wg := sync.WaitGroup{}
	wg.Add(params.Workers)

	estadosCh := make(chan string)
	errosCh := make(chan error, len(params.Estados))

	for i := 0; i < params.Workers; i++ {
		go func(estCh <-chan string, errCh chan<- error) {
			for estado := range estCh {
				err := baixarConfiguracaoDoEstado(params.Saida, estado, params.UsarCache, params.Turno)
				if err != nil {
					errCh <- err
				}
			}
			wg.Done()
		}(estadosCh, errosCh)
	}

	for _, estado := range params.Estados {
		estadosCh <- estado
	}
	close(estadosCh)
	wg.Wait()
	close(errosCh)

	final := linq.FromChannelT(errosCh).AggregateT(func(currErr error, prox error) error {
		return errors.Wrap(currErr, "")
	})

	if final == nil {
		return nil
	}

	return final.(error)
}

func baixarConfiguracaoDoEstado(caminhoAlvo, estado string, usarCache bool, turno int) error {
	var existeCargaDoEstado bool

	codigoTurno := helpers.GetCodigoTurno(turno)
	nomeArquivo := fmt.Sprintf(posfixoNomeCarga, estado, codigoTurno)
	caminhoArquivo := caminhoAlvo + pastaConfiguracoes
	url := fmt.Sprintf(urlAuxModelo, codigoTurno, estado, nomeArquivo)

	_, err := os.Stat(caminhoArquivo + nomeArquivo)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) == false {
			return err
		}
		existeCargaDoEstado = false
	} else {
		existeCargaDoEstado = true
		logger.Infof("O arquivo %s já foi baixado. Pulando download...", nomeArquivo)
	}

	if usarCache == false || existeCargaDoEstado == false {
		logger.Infof("Baixando o arquivo %s", nomeArquivo)

		req, err := http.Get(url)
		if err != nil {
			return err
		}

		bytesCorpo, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}

		tamanhoEmKb := 1024
		logger.Infof("O arquivo %s foi baixado com sucesso. Tamanho %dKB", nomeArquivo, len(bytesCorpo)/tamanhoEmKb)

		err = substituirArquivoDeConfiguracao(caminhoArquivo, nomeArquivo, bytesCorpo)
		if err != nil {
			return err
		}
	}

	return nil
}

func substituirArquivoDeConfiguracao(caminhoArquivo, nomeArquivo string, dados []byte) error {
	err := os.RemoveAll(caminhoArquivo + nomeArquivo)
	if err != nil {
		return err
	}

	err = os.MkdirAll(caminhoArquivo, 0750)
	if err != nil {
		return err
	}
	err = os.WriteFile(caminhoArquivo+nomeArquivo, dados, 0666)
	if err != nil {
		return err
	}

	return nil
}
