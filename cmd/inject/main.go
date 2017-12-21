package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	jsonnet "github.com/google/go-jsonnet"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ghodss/yaml"
)

var (
	script string
)

func main() {
	flag.Parse()

	// read YAML from stdin
	// print YAML to stdout
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(os.Stdin, 4096))
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	// run jsonnet over it
	vm := jsonnet.MakeVM()
	content, err := ioutil.ReadFile(script)
	if err != nil {
		log.Fatal(err)
	}

	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		json, err := yaml.YAMLToJSON(raw)
		if err != nil {
			log.Fatal(err)
		}

		vm.TLACode("o", string(json))
		out, err := vm.EvaluateSnippet(script, string(content))
		yaml, err := yaml.JSONToYAML([]byte(out))
		if err != nil {
			log.Fatal(err)
		}

		if _, err = writer.Write(yaml); err != nil {
			log.Fatal(err)
		}
		if _, err = fmt.Fprint(writer, "---\n"); err != nil {
			log.Fatal(err)
		}
	}
}

func init() {
	flag.StringVar(&script, "script", "inject.jsonnet", "Injection JSONNET script")
}
