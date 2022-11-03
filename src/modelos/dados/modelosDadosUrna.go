package dados

type Auxiliar struct {
	DataGeracao string `json:"dg"`
	HoraGeracao string `json:"hg"`
	F           string `json:"f"`
	Status      string `json:"st"`
	Ds          string `json:"ds"`
	Hashes      []Hash `json:"hashes"`
}

type Hash struct {
	Hash         string   `json:"hash"`
	DataR        string   `json:"dr"`
	HoraR        string   `json:"hr"`
	Status       string   `json:"st"`
	Ds           string   `json:"ds"`
	NomeArquivos []string `json:"nmarq"`
}
