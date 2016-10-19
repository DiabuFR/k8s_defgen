package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const requiredArg = "{REQUIRED}"

type portValues struct {
	Port       uint16
	TargetPort uint16
	Protocol   string
}

type portValuesList struct {
	ports []portValues
}

func (pvl *portValuesList) String() string {
	return fmt.Sprint(pvl.ports)
}

func (pvl *portValuesList) Set(value string) error {
	values := strings.Split(value, ",")
	for _, val := range values {
		info := strings.SplitN(val, ":", 3)

		v, err := strconv.ParseUint(info[0], 10, 16)
		if err != nil {
			return err
		}
		port := portValues{}
		port.Port = uint16(v)

		switch len(info) {
		case 1: // Only Port
			port.TargetPort = port.Port
		case 2: // Port and TargetPort
			v, err = strconv.ParseUint(info[1], 10, 16)
			if err != nil {
				return err
			}
			port.TargetPort = uint16(v)
			port.Protocol = "TCP"
		case 3: // Port, TargetPort and Protocol
			v, err = strconv.ParseUint(info[1], 10, 16)
			if err != nil {
				return err
			}
			port.TargetPort = uint16(v)
			port.Protocol = info[2]
		}
		pvl.ports = append(pvl.ports, port)
	}
	return nil
}

var templateValues = struct {
	Cluster         string
	ClusterProvider string
	ClusterZone     string
	Namespace       string
	Image           string
	Name            string
	Ports           []portValues
}{}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type templateFile struct {
	outFilename string
	tmpl        *template.Template
}

func parseTemplates(filepaths ...string) ([]templateFile, error) {
	files := []templateFile{}

	for _, fp := range filepaths {
		tf := templateFile{}
		tf.outFilename = strings.Replace(filepath.Base(fp), ".tmpl", "", 1)
		file, err := os.Open(fp)
		check(err)

		reader := bufio.NewReader(file)
		firstLine, err := reader.ReadString('\n')
		check(err)

		if !strings.HasPrefix(firstLine, "///") {
			file.Seek(0, 0)
			reader.Reset(file) // Reset position
		} else {
			tf.outFilename = fmt.Sprintf(firstLine[3:len(firstLine)-1], templateValues.Name)
		}
		tmplBytes, err := ioutil.ReadAll(reader)
		check(err)
		tmplString := string(tmplBytes)
		//log.Println(tmplString)
		tf.tmpl, err = template.New(fp).Funcs(funcMap).Parse(tmplString)
		check(err)

		files = append(files, tf)
	}

	return files, nil
}

var funcMap = template.FuncMap{
	"firstRune": func(s string) string {
		return string([]rune(s)[0])
	},
}

func main() {
	flag.StringVar(&templateValues.Cluster, "cluster", requiredArg, "Cluster where definition will be deployed")
	flag.StringVar(&templateValues.Namespace, "namespace", requiredArg, "Namespace where definition will be deployed")
	flag.StringVar(&templateValues.Name, "name", requiredArg, "Name of the resource that'll be deployed")
	flag.StringVar(&templateValues.Image, "img", "", "Docker Image to deploy")

	ports := portValuesList{}
	flag.Var(&ports, "ports", "Ports used by service definitions")
	flag.Parse()

	templateValues.Ports = ports.ports
	clusterInfo := strings.SplitN(templateValues.Cluster, "_", 2)
	templateValues.ClusterProvider = clusterInfo[0]
	templateValues.ClusterZone = clusterInfo[1]

	if templateValues.Cluster == requiredArg ||
		templateValues.Namespace == requiredArg {
		log.Fatal("Missing required parameter")
	}

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Missing template file(s)")
	}

	tmpls, err := parseTemplates(args...)
	check(err)

	outDir := "gen/" + templateValues.Cluster + "/" + templateValues.Namespace + "/" + templateValues.Name + "/"
	os.MkdirAll(outDir, 0777)
	for _, t := range tmpls {
		outFilepath := outDir + t.outFilename
		if _, err = os.Stat(outFilepath); err == nil || !os.IsNotExist(err) {
			check(os.Remove(outFilepath))
		}
		log.Println("Generating", t.tmpl.Name(), "\t->\t", outFilepath)
		f, err := os.Create(outFilepath)
		check(err)
		err = t.tmpl.Execute(f, templateValues)
		check(err)
		f.Sync()
	}
}
