package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const tmpl = `Title:

Description:

Owner:

CC:

Projects:

Priority:

Points:
`

var keysToConduit = map[string]string{
	"title":       "title",
	"description": "description",
	"owner":       "ownerPHID",
	"cc":          "ccPHIDs",
	"priority":    "priority",
	"projects":    "projectPHIDs",
	"points":      "points",
}

var re = regexp.MustCompile(`(?m)^(\w+):`)

func main() {
	log.SetFlags(0)

	// prepare a temp file
	// TODO: add maybe extension
	f, err := ioutil.TempFile("", "task")
	fatalIf(err)

	// try to get the editor otherwise use a default one
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// write our template in it
	_, err = f.WriteString(tmpl)
	fatalIf(err)

	err = f.Close()
	fatalIf(err)

	// open the file in the editor
	cmd := exec.Command(editor, f.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Executing %s %s", editor, f.Name())
	err = cmd.Start()
	fatalIf(err)

	err = cmd.Wait()
	fatalIf(err)

	// sync and seek to 0 so we can read it again
	f, err = os.Open(f.Name())
	fatalIf(err)

	buf, err := ioutil.ReadAll(f)

	// parse the template
	res := re.FindAllIndex(buf, -1)

	// matches is the index where each key starts and its key
	type keyMatch struct {
		index [2]int // start/finish
		key   string
	}
	matches := []keyMatch{}
	for _, m := range res {
		if len(m) != 2 {
			log.Fatal("Invalid index length")
		}

		// get start and beginning and transform it to a string
		s, e := m[0], m[1]-1
		key := string(buf[s:e])

		// store its start and key map
		if key, ok := keysToConduit[strings.ToLower(key)]; ok {
			matches = append(matches, keyMatch{[2]int{s, e}, key})
		}
	}

	// now pair the keys with their values
	data := make(map[string]interface{}, len(matches))
	for i, m := range matches {
		var ends int
		if i == len(matches)-1 {
			ends = len(buf)
		} else {
			ends = matches[i+1].index[0]
		}

		value := strings.TrimSpace(string(buf[m.index[1]+1 : ends]))
		if value != "" {
			data[m.key] = value
		}
	}

	// find phids
	if v, ok := data["ownerPHID"]; ok {
		v, ok := v.(string)
		if !ok {
			fatalf("owner should be a string")
		}

		phids := getPHIDs(v, '@')
		if len(phids) != 1 {
			fatalf("you should specify just 1 owner got: %d", len(phids))
		}

		data["ownerPHID"] = phids[0]
	}

	if v, ok := data["ccPHIDs"]; ok {
		v, ok := v.(string)
		if !ok {
			fatalf("owner should be a string")
		}

		data["ccPHIDs"] = getPHIDs(v, '@')
	}

	if v, ok := data["projectPHIDs"]; ok {
		v, ok := v.(string)
		if !ok {
			fatal("projects should be a string (comma separated if multiple)")
		}

		data["projectPHIDs"] = getPHIDs(v, '#')
	}

	// prepare the json to send to conduit
	buf, err = json.Marshal(data)
	fatalIf(err)

	// the arguments if any to the conduit call
	args := append([]string{"call-conduit"},
		append(os.Args[1:], "maniphest.createtask")...,
	)

	// capture the output
	stdout := bytes.NewBuffer(nil)

	// execute the final command
	cmd = exec.Command("arc", args...)
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(buf)

	log.Print("Sending to conduit ...")
	err = cmd.Start()
	fatalIf(err)

	err = cmd.Wait()
	fatalIf(err)

	{ // decode finally the response
		var res struct {
			Error        string `json:"error"`
			ErrorMessage string `json:"errorMessage"`
			Response     struct {
				URI        string `json:"uri"`
				ObjectName string `json:"objectName"`
			} `json:"response"`
		}

		err := json.NewDecoder(stdout).
			Decode(&res)
		fatalIf(err)

		// if we got an error
		if res.Error != "" || res.ErrorMessage != "" {
			fatal(res.ErrorMessage)
		}

		log.Printf("Task successfully created at %s", res.Response.URI)
	}
}

func getPHIDs(value string, prefix byte) []string {
	// prepare the data
	objects := map[string][]string{
		"names": []string{},
	}

	for _, v := range strings.Split(value, ",") {
		v := strings.TrimSpace(v)
		if v[0] != prefix {
			v = string([]byte{prefix}) + v
		}
		objects["names"] = append(objects["names"], v)
	}

	data, err := json.Marshal(objects)
	fatalIf(err)

	stdout := bytes.NewBuffer(nil)
	cmd := exec.Command("arc", "call-conduit", "phid.lookup")
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(data)
	err = cmd.Start()
	fatalIf(err)
	err = cmd.Wait()
	fatalIf(err)

	var res struct {
		Error        string                       `json:"error"`
		ErrorMessage string                       `json:"errorMessage"`
		Response     map[string]map[string]string `json:"response"`
	}

	err = json.NewDecoder(stdout).Decode(&res)
	fatalIf(err)

	// if we got an error
	if res.Error != "" || res.ErrorMessage != "" {
		fatal(res.ErrorMessage)
	}

	phids := make([]string, len(res.Response))
	for i, o := range objects["names"] {
		v, ok := res.Response[o]
		if !ok {
			fatalf("unable to find the phid of %q", o)
		}
		phids[i] = v["phid"]
	}

	return phids
}

func fatalIf(err error) {
	if err == nil {
		return
	}
	fatalWithCaller(err)
}

func fatalf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	fatalWithCaller(err)
}

func fatal(msg string) {
	err := errors.New(msg)
	fatalWithCaller(err)
}

func fatalWithCaller(err error) {
	fileline := "<unknown>"
	_, file, line, ok := runtime.Caller(2)
	if ok {
		fileline = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	log.Fatalf("%s %v", fileline, err)
}
