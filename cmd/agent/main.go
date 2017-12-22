package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	jsonnet "github.com/google/go-jsonnet"
)

func main() {
	flag.Parse()
	vm := jsonnet.MakeVM()
	content, err := ioutil.ReadFile("bootstrap.jsonnet")
	if err != nil {
		log.Fatal(err)
	}
	vm.TLAVar("ads_host", ads)
	vm.TLAVar("id", fmt.Sprintf("%s/%s", os.Getenv("POD_NAMESPACE"), os.Getenv("POD_NAME")))
	out, err := vm.EvaluateSnippet(script, string(content))
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile("bootstrap.json", []byte(out), 0644)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command(envoy, "-c", "bootstrap.json", "-l", "info", "--drain-time-s", "1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

var (
	envoy  string
	ads    string
	script string
)

func init() {
	flag.StringVar(&envoy, "envoy", "/usr/local/bin/envoy", "Envoy binary")
	flag.StringVar(&ads, "ads", "envoycontroller", "Envoy mesh controller host address")
	flag.StringVar(&script, "script", "bootstrap.jsonnet", "bootstrap script")
}
