// +build linter

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	helmChart = "helm/sealed-secrets"
)

type Flags struct {
	versionFile string
}

func (f *Flags) Bind(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&f.versionFile, "config", "version.yaml", "Version config yaml")
}

type Version struct {
	Version      string    `yaml:"version"`
	HelmRevision string    `yaml:"helmRevision"`
	HelmReplace  Replacers `yaml:"helmReplace"`
}

type Replacers []Replacer

func (rs Replacers) Replace(src string) string {
	for _, r := range rs {
		src = r.Replace(src)
	}
	return src
}

type Replacer struct {
	Src string `yaml:"replace"`
	Dst string `yaml:"with"`
}

func (pr Replacer) Replace(src string) string {
	return strings.Replace(src, pr.Src, pr.Dst, -1)
}

func (v Version) HelmVersion() string {
	tmp := fmt.Sprintf("%s-%s", v.Version, v.HelmRevision)
	return v.HelmReplace.Replace(tmp)
}

func readVersion(filename string) (v Version, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return v, err
	}
	defer f.Close()
	err = yaml.NewDecoder(f).Decode(&v)
	return
}

type Chart struct {
	Metadata ChartMetadata
}

type ChartMetadata struct {
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

func loadHelmChart(dir string) (Chart, error) {
	var m ChartMetadata
	f, err := os.Open(filepath.Join(dir, "Chart.yaml"))
	if err != nil {
		return Chart{}, err
	}
	if err := yaml.NewDecoder(f).Decode(&m); err != nil {
		return Chart{}, err
	}
	return Chart{
		Metadata: m,
	}, nil
}

func mainE(flags Flags) error {
	v, err := readVersion(flags.versionFile)
	if err != nil {
		return err
	}
	y, err := yaml.Marshal(&v)
	log.Printf("Ensuring repo is consistently at version:\n\n%s\n\n", y)

	ch, err := loadHelmChart(helmChart)
	if err != nil {
		return err
	}
	if got, want := v.HelmVersion(), ch.Metadata.Version; got != want {
		log.Fatalf("Helm chart version: got: %q, want: %q", got, want)
	}
	if got, want := v.Version, ch.Metadata.AppVersion; got != want {
		log.Fatalf("Helm app version: got: %q, want: %q", got, want)
	}
	log.Printf("ok")
	return nil
}

func main() {
	var flags Flags
	flags.Bind(nil)
	flag.Parse()

	if err := mainE(flags); err != nil {
		log.Print(err)
	}
}
