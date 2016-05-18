package pillar

var Conf struct {
	DatabaseName string `default:"eve"`
	DatabaseHost string `default:"localhost:27017"`
	Listen       string `default:":5002"`
}
