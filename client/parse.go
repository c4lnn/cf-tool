package client

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/c4lnn/cf-tool/util"

	"github.com/k0kubun/go-ansi"

	"github.com/fatih/color"
)

func findSample(body []byte) (input [][]byte, output [][]byte, err error) {
	irg := regexp.MustCompile(`class="input"([\s\S]*?)<pre([\s\S]*?)>([\s\S]*?)</pre>`)
	org := regexp.MustCompile(`class="output"([\s\S]*?)<pre([\s\S]*?)>([\s\S]*?)</pre>`)
	a := irg.FindAllSubmatch(body, -1)
	b := org.FindAllSubmatch(body, -1)
	if a == nil || b == nil || len(a) != len(b) {
		return nil, nil, fmt.Errorf("Cannot parse sample with input %v and output %v", len(a), len(b))
	}
	irg = regexp.MustCompile(`class="test-example-line([\s\S]*?)>([\s\S]*?)</div>`)
	filter := func(src []byte) []byte {
		s := html.UnescapeString(string(src))
		return []byte(strings.TrimSpace(s) + "\n")
	}
	for i := 0; i < len(a); i++ {
		line := irg.FindAllSubmatch(a[i][3], -1)
		var temp []byte
		for j := 0; j < len(line); j++ {
			temp = append(temp, filter(line[j][2])...)
		}
		if line == nil {
			temp = append(temp, filter(a[i][3])...)
		}
		input = append(input, temp)
		output = append(output, filter(b[i][3]))
	}
	return
}

// ParseProblem parse problem to path. mu can be nil
func (c *Client) ParseProblem(URL, path string, mu *sync.Mutex) (samples int, standardIO bool, err error) {
	body, err := util.GetBody(c.client, URL)
	if err != nil {
		return
	}

	_, err = findHandle(body)
	if err != nil {
		return
	}

	input, output, err := findSample(body)
	if err != nil {
		return
	}

	standardIO = true
	if !bytes.Contains(body, []byte(`<div class="sample-test"`)) {
		standardIO = false
	}

	for i := 0; i < len(input); i++ {
		fileIn := filepath.Join(path, fmt.Sprintf("in%v.txt", i+1))
		fileOut := filepath.Join(path, fmt.Sprintf("ans%v.txt", i+1))
		e := os.WriteFile(fileIn, input[i], 0644)
		if e != nil {
			if mu != nil {
				mu.Lock()
			}
			color.Red(e.Error())
			if mu != nil {
				mu.Unlock()
			}
		}
		e = os.WriteFile(fileOut, output[i], 0644)
		if e != nil {
			if mu != nil {
				mu.Lock()
			}
			color.Red(e.Error())
			if mu != nil {
				mu.Unlock()
			}
		}
	}
	return len(input), standardIO, nil
}

// Parse parse
func (c *Client) Parse(info Info) (problems []string, paths []string, err error) {
	color.Cyan("Parse " + info.Hint())

	problemID := info.ProblemID
	info.ProblemID = "%v"
	urlFormatter, err := info.ProblemURL(c.host)
	if err != nil {
		return
	}
	info.ProblemID = ""
	if problemID == "" {
		statics, err := c.Statis(info)
		if err != nil {
			return nil, nil, err
		}
		problems = make([]string, len(statics))
		for i, problem := range statics {
			problems[i] = problem.ID
		}
	} else {
		problems = []string{problemID}
	}
	contestPath := info.Path()
	ansi.Printf(color.CyanString("The problem(s) will be saved to %v\n"), color.GreenString(contestPath))

	wg := sync.WaitGroup{}
	wg.Add(len(problems))
	mu := sync.Mutex{}
	paths = make([]string, len(problems))
	for i, problemID := range problems {
		paths[i] = filepath.Join(contestPath, strings.ToUpper(problemID))
		go func(problemID, path string) {
			defer wg.Done()
			mu.Lock()
			fmt.Printf("Parsing %v\n", problemID)
			mu.Unlock()

			err = os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return
			}
			URL := fmt.Sprintf(urlFormatter, problemID)

			samples, standardIO, err := c.ParseProblem(URL, path, &mu)
			if err != nil {
				return
			}

			warns := ""
			if !standardIO {
				warns = color.YellowString("Non standard input output format.")
			}
			mu.Lock()
			if err != nil {
				color.Red("Failed %v. Error: %v", problemID, err.Error())
			} else {
				ansi.Printf("%v %v\n", color.GreenString("Parsed %v with %v samples.", problemID, samples), warns)
			}
			mu.Unlock()
		}(problemID, paths[i])
	}
	wg.Wait()
	return
}
