package modelos

import "github.com/sirupsen/logrus"

type Parametros struct {
	UsarCache   bool         `arg:"-c, --cache" help:"Define se deve-se pular o download de arquivos já presentes na pasta, caso não esteja presente o download será feito novamente e o arquivo substituído"`
	Estados     []string     `arg:"positional, required" help:"Os estados dos quais serão baixados os arquivos e configurações, pelo menos um deve ser especificado ou 'all' para todos. Ex: es ac sp"`
	Workers     int          `arg:"-w, --workers" placeholder:"N" help:"A quantidade de downloads simultâneos. Note que número altos não significam que seu sistema conseguirá disparar tantos dowloads ao mesmo tempo"`
	Saida       string       `arg:"-s, --saida" default:"./arquivos/" placeholder:"CAMINHO" help:"O local que deseja usar de raíz para se salvar os arquivos. Pode ser caminho absoluto ou relativo ao diretório de onde esse programa é executado"`
	Verbosidade logrus.Level `arg:"-v, --verbosidade" placeholder:"NÍVEL" help:"Quantos logs devem ser exibidos. Em ordem de criticidade (0 à 6): panic > fatal > error > warn > info > debug > trace"`
	Zip         bool         `arg:"-z, --zip" help:"Se presente, o programa efetuara a compressão da pasta do estado"`
	Turno       int          `arg:"-t, --turno" help:"Define de qual turno será efetuado o download"`
}
