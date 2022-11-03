package configuracao

type Estado struct {
	DataGeracao string        `json:"dg"`
	HoraGeracao string        `json:"hg"`
	F           string        `json:"f"`   // Não sei o que isso pode significar
	CDP         string        `json:"cdp"` // Não sei o que isso pode significar
	Abrangencia []DadosEstado `json:"abr"`
}

type DadosEstado struct {
	Codigo     string           `json:"cd"`
	Nome       string           `json:"ds"`
	Municipios []DadosMunicipio `json:"mu"`
}

type DadosMunicipio struct {
	Codigo string      `json:"cd"`
	Nome   string      `json:"nm"`
	Zonas  []DadosZona `json:"zon"`
}

type DadosZona struct {
	Codigo string       `json:"cd"`
	Secoes []DadosSecao `json:"sec"`
}

type DadosSecao struct {
	Numero string `json:"ns"`
	NSP    string `json:"nsp"` // Não o que pode significar, mas não parece diferir de 'ns'
}
