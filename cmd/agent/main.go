package main

import (
	"flag"
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
	vm.TLAVar("id", id)
	out, err := vm.EvaluateSnippet(script, string(content))
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(config, []byte(out), 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("id %q, cluster %q", id, cluster)

	cmd := exec.Command(envoy, "-c", config, "--v2-config-only", "-l", "info", "--drain-time-s", "1")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

var (
	envoy   string
	config  string
	ads     string
	script  string
	id      string
	cluster string
)

func init() {
	flag.StringVar(&envoy, "envoy", "/usr/local/bin/envoy", "Envoy binary")
	flag.StringVar(&config, "config", "/tmp/bootstrap.json", "Envoy config output")
	flag.StringVar(&ads, "ads", "localhost", "Envoy mesh controller host address")
	flag.StringVar(&script, "script", "bootstrap.jsonnet", "bootstrap script")
	flag.StringVar(&id, "id", "unknown-id", "Workload ID")
	flag.StringVar(&cluster, "cluster", "unknown-cluster", "Service cluster")
}
