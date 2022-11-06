package urna

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"tse-scrapper/helpers"
	"tse-scrapper/jsonHelpers"
	"tse-scrapper/modelos"
	"tse-scrapper/modelos/configuracao"
	"tse-scrapper/modelos/dados"

	"github.com/MisterKaiou/go-functional/result"
	"github.com/MisterKaiou/go-functional/unit"
	"github.com/pkg/errors"
	logger "github.com/sirupsen/logrus"
)

type infoSecoes struct {
	estado, nmMunicipio, cdMunicipio, zona, secao, nomeArquivoAux string
}

const (
	modeloUrlDownloadArquivo = "https://resultados.tse.jus.br/oficial/ele2022/arquivo-urna/%s/dados/%s/%05s/%04s/%04s/%s/%s"
	nomeArquivoAuxiliar      = "p000%s-%s-m%05s-z%04s-s%04s-aux.json"
	modeloUrlArquivoAuxiliar = "https://resultados.tse.jus.br/oficial/ele2022/arquivo-urna/%s/dados/%s/%05s/%04s/%04s/%s"

	pastaDadosUrna = "dados/"
)

func BaixarDadosDaUrna(params modelos.Parametros) {
	wWg, eWg := sync.WaitGroup{}, sync.WaitGroup{}
	wWg.Add(params.Workers)
	eWg.Add(1)

	infoSecCh := make(chan infoSecoes, params.Workers*100)
	errosCh := make(chan error)

	go func(ch chan error) {
		for err := range ch {
			logger.Error(err)
		}
		eWg.Done()
	}(errosCh)

	for i := 0; i < params.Workers; i++ {
		go baixarArquivosSecoes(params.Saida, params.UsarCache, params.Turno, infoSecCh, errosCh, &wWg)
	}

	prepararInfoArquivosSecoes(params.Saida, params.Estados, params.Turno, infoSecCh, errosCh)

	close(infoSecCh)
	wWg.Wait()
	close(errosCh)
	eWg.Wait()

	if params.Zip {
		for _, estado := range params.Estados {
			logger.Warnf("Comprimindo pasta do estado de [%s]", strings.ToUpper(estado))

			fonte := params.Saida + pastaDadosUrna + estado
			alvo := fonte + ".zip"

			err := os.RemoveAll(alvo)
			if err != nil {
				logger.Error(err)
				return
			}

			file, err := os.Create(alvo)
			if err != nil {
				logger.Error(err)
				return
			}

			writer := zip.NewWriter(file)

			err = filepath.Walk(fonte, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}

				header, err := zip.FileInfoHeader(info)
				if err != nil {
					return err
				}

				header.Method = zip.Deflate

				header.Name, err = filepath.Rel(filepath.Dir(fonte), path)
				if err != nil {
					return err
				}
				if info.IsDir() {
					header.Name += "/"
				}

				headerWriter, err := writer.CreateHeader(header)
				if err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				f, err := os.Open(path)
				if err != nil {
					return err
				}

				_, err = io.Copy(headerWriter, f)
				if err != nil {
					return err
				}

				err = f.Close()

				return err
			})

			if err != nil {
				logger.Error(err)
				return
			}

			err = writer.Close()
			if err != nil {
				logger.Error(err)
			}

			err = file.Close()
			if err != nil {
				logger.Error(err)
			}

			logger.Warnf("Deletando a pasta do estado [%s] após a compressão", strings.ToUpper(estado))
			err = os.RemoveAll(fonte)
			if err != nil {
				logger.Error(err)
				return
			}
		}
	}
}

func prepararInfoArquivosSecoes(saida string, estados []string, turno int, infoSecCh chan infoSecoes, errosCh chan<- error) {
	for _, estado := range estados {
		arquivoConfig := lerArquivoConfiguracao(saida, estado, turno)
		resultConfigEstado := result.Bind(arquivoConfig, func(b []byte) result.Of[configuracao.Estado] {
			return result.FromTupleOf(jsonHelpers.DesserializarJson[configuracao.Estado](b))
		})

		if resultConfigEstado.IsError() {
			errosCh <- resultConfigEstado.UnwrapError()
			return
		}

		configEstado := resultConfigEstado.Unwrap()
		qtdMunicipios := len(configEstado.Abrangencia[0].Municipios)
		for i, municipio := range configEstado.Abrangencia[0].Municipios { // Abrangência sempre tem somente uma entrada que é sempre o estado da configuração
			for _, zona := range municipio.Zonas {
				for _, secao := range zona.Secoes {
					infoSecCh <- infoSecoes{
						estado:      estado,
						cdMunicipio: municipio.Codigo,
						nmMunicipio: municipio.Nome,
						zona:        zona.Codigo,
						secao:       secao.Numero,
					}
				}
			}

			logger.Warnf(
				"Enfileiramento para download das seções do município de [%s] em [%s] finalizado. [%d] de [%d]",
				municipio.Nome, strings.ToUpper(estado), i+1, qtdMunicipios)
		}
	}
}

func baixarArquivosSecoes(
	caminho string, usarCache bool, turno int, infoSecCh <-chan infoSecoes, errCh chan<- error, wg *sync.WaitGroup) {
	for info := range infoSecCh {
		var bytesArq result.Of[[]byte]
		var arquivoAux result.Of[dados.Auxiliar]

		totalBaixado := 0
		url, nomeArquivo := construirNomeEUrlArquivoAuxiliar(info.estado, info.cdMunicipio, info.zona, info.secao, turno)
		info.nomeArquivoAux = nomeArquivo
		existe, err := arquivoDeDadosExiste(caminho, info.nomeArquivoAux, info)
		if err != nil {
			errCh <- err
		} else if existe && usarCache {
			logger.Debugf("O arquivo auxiliar %s já foi baixado. Pulando download...", nomeArquivo)
			bytesArq = lerArquivoAuxiliar(caminho, info)
			arquivoAux = result.Bind(bytesArq, func(b []byte) result.Of[dados.Auxiliar] {
				return result.FromTupleOf(jsonHelpers.DesserializarJson[dados.Auxiliar](b))
			})

			if arquivoAux.IsError() {
				logger.Errorf("Falha em ler o arquivo %s, efetuando download novamente.", nomeArquivo)

				bytesArq = baixarArquivoAuxiliar(url)
				res := result.Bind(bytesArq, func(b []byte) result.Of[unit.Unit] {
					totalBaixado += len(b)

					return salvarArquivoDeDados(caminho, info.nomeArquivoAux, b, info)
				})
				arquivoAux =
					result.Flatten(result.CombineBy(res, bytesArq, func(b []byte, _ unit.Unit) result.Of[dados.Auxiliar] {
						return result.FromTupleOf(jsonHelpers.DesserializarJson[dados.Auxiliar](b))
					}))
			}

		} else {
			bytesArq = baixarArquivoAuxiliar(url)
			res := result.Bind(bytesArq, func(b []byte) result.Of[unit.Unit] {
				totalBaixado += len(b)

				return salvarArquivoDeDados(caminho, info.nomeArquivoAux, b, info)
			})
			arquivoAux =
				result.Flatten(result.CombineBy(res, bytesArq, func(b []byte, _ unit.Unit) result.Of[dados.Auxiliar] {
					return result.FromTupleOf(jsonHelpers.DesserializarJson[dados.Auxiliar](b))
				}))
		}

		resultadosDownloads := result.Map(arquivoAux, func(a dados.Auxiliar) []result.Of[unit.Unit] {
			resultados := make([]result.Of[unit.Unit], 0)

			for _, hash := range a.Hashes {
				for _, nomeArquivo := range hash.NomeArquivos {
					dadosExiste, err := arquivoDeDadosExiste(caminho, nomeArquivo, info)
					if err != nil {
						errCh <- err
					} else if dadosExiste && usarCache {
						logger.Debugf("O arquivo de urna %s já foi baixado. Pulando download...", nomeArquivo)
						continue
					}

					arqUrna := baixarArquivoDeUrna(nomeArquivo, info, hash.Hash, turno)
					resSalvamento := result.Bind(arqUrna, func(b []byte) result.Of[unit.Unit] {
						totalBaixado += len(b)
						return salvarArquivoDeDados(caminho, nomeArquivo, b, info)
					})

					resultados = append(resultados, resSalvamento)
				}
			}

			return resultados
		})

		if resultadosDownloads.IsError() {
			errCh <- resultadosDownloads.UnwrapError()
		} else {
			for _, r := range resultadosDownloads.Unwrap() {
				if r.IsError() {
					errCh <- r.UnwrapError()
				}
			}
		}

		logger.Infof(
			"Download dos arquivos da seção [%s] na zona eleitoral [%s] da cidade de [%s] em [%s] concluido. Total Baixado [%dKB]",
			info.secao, info.zona, strings.ToTitle(strings.ToLower(info.nmMunicipio)), strings.ToUpper(info.estado), totalBaixado/1024)
	}

	wg.Done()
}

func baixarArquivoDeUrna(nomeArq string, info infoSecoes, hash string, turno int) result.Of[[]byte] {
	codigoTurno := helpers.GetCodigoTurno(turno)
	url := fmt.Sprintf(modeloUrlDownloadArquivo, codigoTurno, info.estado, info.cdMunicipio, info.zona, info.secao, hash, nomeArq)
	response := result.FromTupleOf(http.Get(url))
	return result.Bind(response, func(res *http.Response) result.Of[[]byte] {
		return result.FromTupleOf(io.ReadAll(res.Body))
	})
}

func baixarArquivoAuxiliar(url string) result.Of[[]byte] {
	response := result.FromTupleOf(http.Get(url))
	return result.Bind(response, func(res *http.Response) result.Of[[]byte] {
		return result.FromTupleOf(io.ReadAll(res.Body))
	})
}

func lerArquivoAuxiliar(caminho string, info infoSecoes) result.Of[[]byte] {
	caminhoFinal := construirCaminhoFinalArquivoDados(caminho, info.estado, info.cdMunicipio, info.nmMunicipio, info.zona, info.secao)
	return result.FromTupleOf(os.ReadFile(caminhoFinal + info.nomeArquivoAux))
}

func arquivoDeDadosExiste(caminho, nomeArq string, info infoSecoes) (bool, error) {
	caminhoFinal := construirCaminhoFinalArquivoDados(caminho, info.estado, info.cdMunicipio, info.nmMunicipio, info.zona, info.secao)

	_, err := os.Stat(caminhoFinal + nomeArq)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) == false {
			return false, err
		}
		return false, nil
	} else {
		return true, nil
	}
}

func salvarArquivoDeDados(caminho, nomeArq string, conteudo []byte, info infoSecoes) result.Of[unit.Unit] {
	caminhoFinal := construirCaminhoFinalArquivoDados(
		caminho,
		info.estado,
		info.cdMunicipio,
		info.nmMunicipio,
		info.zona,
		info.secao)

	res := result.FromTupleOf(unit.Unit{}, os.RemoveAll(caminhoFinal+nomeArq))

	res = result.Bind(res, func(u unit.Unit) result.Of[unit.Unit] {
		return result.FromTupleOf(u, os.MkdirAll(caminhoFinal, 0750))
	})
	return result.Bind(res, func(u unit.Unit) result.Of[unit.Unit] {
		return result.FromTupleOf(
			u,
			os.WriteFile(caminhoFinal+nomeArq, conteudo, 0666))
	})
}

func construirCaminhoFinalArquivoDados(caminho, estado, cdMunicipio, nmMunicipio, cdZona, cdSecao string) string {
	return caminho +
		pastaDadosUrna +
		estado + "/" +
		cdMunicipio + " - " + nmMunicipio + "/" +
		cdZona + "/" +
		cdSecao + "/"
}

func lerArquivoConfiguracao(caminhoArquivos, estado string, turno int) result.Of[[]byte] {
	return result.FromTupleOf(
		os.ReadFile(caminhoArquivos + pastaConfiguracoes + fmt.Sprintf(posfixoNomeCarga, estado, helpers.GetCodigoTurno(turno))))
}

func construirNomeEUrlArquivoAuxiliar(estado, cdMunicipio, cdZona, cdSecao string, turno int) (string, string) {
	codigoTurno := helpers.GetCodigoTurno(turno)
	nomeArquivo := fmt.Sprintf(nomeArquivoAuxiliar, codigoTurno, estado, cdMunicipio, cdZona, cdSecao)
	url := fmt.Sprintf(modeloUrlArquivoAuxiliar, codigoTurno, estado, cdMunicipio, cdZona, cdSecao, nomeArquivo)
	return url, nomeArquivo
}
