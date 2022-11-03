package main

import (
	"strings"
	"tse-scrapper/modelos"
	"tse-scrapper/urna"

	l "github.com/ahmetb/go-linq/v3"
	"github.com/alexflint/go-arg"
	nested "github.com/antonfisher/nested-logrus-formatter"
	logger "github.com/sirupsen/logrus"
)

var siglasEstados = []string{
	"ac", "al", "ap", "am",
	"ba",
	"ce",
	"df",
	"es",
	"go",
	"ma", "mt", "ms", "mg",
	"pa", "pb", "pr", "pe", "pi",
	"rj", "rn", "rs", "ro", "rr",
	"sc", "sp", "se",
	"to",
}

func main() {
	args := modelos.Parametros{Workers: 1, Turno: 1, Verbosidade: logger.WarnLevel}
	arg.MustParse(&args)
	logger.SetLevel(args.Verbosidade)
	logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})
	args.Estados = normalizarParametrosEstado(args.Estados)

	err := urna.BaixarConfiguracaoDosEstados(args)
	sairSeErro(err)

	urna.BaixarDadosDaUrna(args)

	logger.Infof("Download dos arquivos dos estados %v finalizado", args.Estados)
}

func sairSeErro(err error) {
	if err != nil {
		panic(err)
	}
}

func normalizarParametrosEstado(parametros []string) []string {
	if parametros[0] == "all" {
		logger.Info("Nenhum estado escolhido. Todos serão usados")
		return siglasEstados
	} else {
		var invalidos []string

		l.From(parametros).SelectT(func(est string) string {
			return strings.ToLower(est)
		}).ToSlice(&parametros)

		l.From(parametros).WhereT(
			func(estado string) bool {
				return l.From(siglasEstados).Contains(estado) == false
			}).ToSlice(&invalidos)

		if len(invalidos) > 0 {
			logger.Fatalf("Os estados %v são inválidos", invalidos)
		}

		logger.Infof("Os estados %v serão usados", parametros)
		return parametros
	}
}
