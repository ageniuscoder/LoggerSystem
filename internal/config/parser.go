package config

type Parser interface {
	Parse(data []byte) (*LoggerConfig, error)
}

var parsers = map[string]Parser{} //perfer it instead of singleton factory

func Register(ext string, p Parser) {
	if _, ok := parsers[ext]; !ok {
		parsers[ext] = p
	}
}

func getParser(ext string) (Parser, bool) {
	if p, ok := parsers[ext]; ok {
		return p, true
	}
	return nil, false
}