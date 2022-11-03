package helpers

import logger "github.com/sirupsen/logrus"

func GetCodigoTurno(turno int) string {
	switch turno {
	case 1:
		return "406"
	case 2:
		return "407"
	default:
		logger.Panicf("Turno inválido")
		return ""
	}
}
