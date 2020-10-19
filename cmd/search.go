// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	//"sort"
	"strings"
	"time"
)


var configFile string
var project string
var symbol string
var def string
var _ string
var text string
var workfile string
var _ string
//var opengork_history_avail bool
//var opengrok_history_timestamp int64

var homeCache , _ = os.UserCacheDir()

var opengrokHistoryDir = path.Join(homeCache, "opengrok_history")
const diffTime = 3600 * 24 * 30

type FilePrio struct {
	Path string
	Prio int
	Results string
}

func deferTime(t time.Time)  {
	fmt.Printf("Elapsed time: %s\n", time.Since(t))
}

func deferClearCache() {
	nowTime := time.Now().Unix()
	_ = filepath.Walk(opengrokHistoryDir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if (nowTime - info.ModTime().Unix()) > diffTime {
			_ = os.Remove(path)
		}

		return nil
	})
}

func calPrio(target string, workfile string) int {
	var matched int
	var count int
	if len(workfile) > 0 {
		if len(workfile) < len(target) {
			count = len(workfile)
		} else {
			count = len(target)
		}
		for matched = 0; matched < count; matched++ {
			if workfile[matched] != target[matched] {
				break
			}
		}
		targetBaseName := path.Base(target)
		wfBaseName := path.Base(workfile)
		if len(targetBaseName) > len(wfBaseName) {
			count = len(wfBaseName)
		} else {
			count = len(targetBaseName)
		}
		var baseMatched int = 0
		for baseMatched = 0; baseMatched < count; baseMatched++ {
			if targetBaseName[baseMatched] != wfBaseName[baseMatched] {
				break
			}
		}
		matched = matched + baseMatched
	} else {
		matched = len(target)
	}

	if strings.Contains(strings.ToLower(target), "/test") {
		matched = matched - 100
	}

	return -matched
}

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "search opengrok with rest API",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println("search called")
		defer deferTime(time.Now())
		defer deferClearCache()
		var searchWithType = false
		files := make(chan FilePrio, 10)
		var wg sync.WaitGroup

		// one reqeust and notify WaitGroup done
		requestFunc := func (files chan<- FilePrio, requestUrl string, sourceRoot string, workfile string, wg* sync.WaitGroup) {
			requestOpengrok(files, requestUrl, sourceRoot, workfile)
			wg.Done()
		}

		sourceRoot := ""
		buf, err := ioutil.ReadFile(configFile)
		if err != nil {
			_ = fmt.Errorf("Failed to read %s, %v\n", configFile, err)
			os.Exit(7)
		}
		openworkXml := strings.Split(string(buf), "\n")
		for i, s := range openworkXml {
			if strings.Contains(s, "<void property=\"sourceRoot\">") {
				sourceRoot = openworkXml[i+1]
				break
			}
		}
		sourceRoot = strings.Split(sourceRoot, ">")[1]
		sourceRoot = strings.Split(sourceRoot, "<")[0]
		//fmt.Printf("sourceRoot : %s\n", sourceRoot)
		if len(sourceRoot) == 0 {
			_ = fmt.Errorf("sourceRoot not found\n")
			os.Exit(6)
		}

		var requestUrl = "http://127.0.0.1:8080/source/api/v1/search?"
		if len(project) != 0 {
			requestUrl = requestUrl + "projects=" + project
		} else {
			os.Exit(1)
		}
		if len(configFile) == 0 {
			os.Exit(5)
		}
		if len(symbol) != 0 {
			searchWithType = true
			_ = symbol
			requestUrl = requestUrl + "&symbol=" + url.QueryEscape(symbol)
		}
		if len(def) != 0 {
			searchWithType = true
			_ = def
			requestUrl = requestUrl + "&def=" + url.QueryEscape(def)
		}
		if len(text) != 0 {
			//fmt.Printf("%s\n",text)
			searchWithType = false
			_ = text
			requestUrl = requestUrl + "&full=" + url.QueryEscape(text)
		}
		historyKey := requestUrl
		if len(workfile) != 0 {
			fileSuffix := path.Ext(path.Base(workfile))
			historyKey = historyKey + "&type=" + fileSuffix
		}
		var keys []FilePrio
		keysLoaded, success := loadHistory(historyKey, sourceRoot)

		if success == false {
			if searchWithType == true && len(workfile) != 0 {
				var taper = ""
				if strings.HasSuffix(workfile, ".java") {
					taper = "&type=java"
					wg.Add(1)
					go requestFunc(files, requestUrl+taper, sourceRoot, workfile, &wg)
				} else if strings.HasSuffix(workfile, ".cpp") || strings.HasSuffix(workfile, ".hpp") ||
					strings.HasSuffix(workfile, ".h") || strings.HasSuffix(workfile, ".cc") {
					wg.Add(3)
					go requestFunc(files, requestUrl+"&type=cxx", sourceRoot, workfile, &wg)
					go requestFunc(files, requestUrl+"&type=c", sourceRoot, workfile, &wg)
					go requestFunc(files, requestUrl+"&type=asm", sourceRoot, workfile, &wg)
				} else if strings.HasSuffix(workfile, ".c") {
					wg.Add(2)
					go requestFunc(files, requestUrl+"&type=c", sourceRoot, workfile, &wg)
					go requestFunc(files, requestUrl+"&type=asm", sourceRoot, workfile, &wg)
				} else if strings.HasSuffix(workfile, ".go") {
					wg.Add(1)
					go requestFunc(files, requestUrl+"&type=golang", sourceRoot, workfile, &wg)
				} else if strings.HasSuffix(workfile, ".py") {
					wg.Add(1)
					go requestFunc(files, requestUrl+"&type=python", sourceRoot, workfile, &wg)
				} else {
					wg.Add(1)
					go requestFunc(files, requestUrl, sourceRoot, workfile, &wg)
				}
				//request_url = request_url + taper

			} else {
				wg.Add(1)
				go requestFunc(files, requestUrl, sourceRoot, workfile, &wg)
			}

			// Waiting all requests done to close the Channel
			go func(files chan<- FilePrio, wg *sync.WaitGroup) {
				wg.Wait()
				close(files)
			}(files, &wg)
			//fmt.Println(request_url)

			// Read results from channel of all requests


		} else {

			// resort the loaded data
			if len(workfile) > 0 {
				if sourceRoot[len(sourceRoot)-1] != '/' {
					workfile = workfile[len(sourceRoot):]
				} else {
					workfile = workfile[len(sourceRoot)-1 :]
				}
			}
			// re-calculate prio
			go func(keys []FilePrio, workfile string, files chan<- FilePrio) {
				var wg sync.WaitGroup
				for _, key := range keys {
					wg.Add(1)
					go func(key FilePrio, workfile string, wg *sync.WaitGroup, files chan<- FilePrio) {
						defer wg.Done()
						key.Prio = calPrio(key.Path, workfile)
						//fmt.Printf("%v\n", key)
						files <- key
					}(key, workfile, &wg, files)
				}
				wg.Wait()
				close(files)
			}(keysLoaded, workfile, files)
		}

		// read FilePrio from files , maybe from cached file or opengrok
		for f := range files {
			keys = append(keys, f)
		}

		// sort
		sort.SliceStable(keys, func(i, j int) bool {
			return keys[i].Prio < keys[j].Prio
		})

		// dump
		//for k, v := range results {
		for _, key := range keys {
				//fmt.Printf(key.path + "\n")
				fmt.Printf(key.Results)
		}

		fmt.Printf("\n")
		if success == false {
			saveHistory(historyKey, keys)
		}

			//sort.Strings(keylist)
			//for _, key := range keylist {
			//	fmt.Printf("%s %s\n", key, sortResult[key])
			//}


		//fmt.Printf("\nOpengrok time: %fms\n", searchTime)

	},
}

func md5v(historyKey string) string {
	h := md5.New()
	h.Write([]byte(historyKey))
	return hex.EncodeToString(h.Sum(nil))
}

func loadHistory(historyKey string, sourceRoot string) ([]FilePrio, bool) {
	getExecpathMtime := func() int64 {
		var exepath string
		var err error
		if exepath, err = exec.LookPath(os.Args[0]); err != nil {
			return 0
		}
		if exepath, err = filepath.Abs(exepath); err != nil {
			return 0
		}
		if exepath, err = filepath.EvalSymlinks(exepath); err != nil {
			return 0
		}
		if stat, err := os.Lstat(exepath); err == nil {
			return stat.ModTime().Unix()
		}

		return 0
	}
	var keys []FilePrio
	md5Key := md5v(historyKey)
	historyPath := path.Join(opengrokHistoryDir, md5Key)
	s, err := os.Lstat(historyPath)
	if os.IsNotExist(err) {
		return nil, false
	}

	//if (time.Now().Unix() - s.ModTime().Unix() > 60 * 60 * 8) {
	//	return nil, false
	//}
	opengrokHistoryTimestamp := s.ModTime().Unix()
	if opengrokHistoryTimestamp < getExecpathMtime() {
		return nil, false
	}

	c, err := ioutil.ReadFile(historyPath)
	if err != nil {
		return nil, false
	}

	//fmt.Printf(string(c))
	err = json.Unmarshal(c, &keys)
	if err != nil {
		fmt.Printf("Failed to json unmarshal %v\n", err)
		return nil, false
	}

	for _, key := range keys {
		path := path.Join(sourceRoot ,key.Path)
		s, err := os.Lstat(path)
		if os.IsNotExist(err) {
			return nil, false
		}
		if s.ModTime().Unix() > opengrokHistoryTimestamp {
			return nil, false
		}
	}

	_ = os.Chtimes(historyPath, time.Now(), time.Now())

	fmt.Printf("\nloaded from %s\n", historyPath)
	return keys, true
}

func saveHistory(historyKey string, keys []FilePrio) bool {
	md5Key := md5v(historyKey)
	_ = os.Mkdir(opengrokHistoryDir, os.ModePerm)
	historyPath := path.Join(opengrokHistoryDir, md5Key)
	f, err := os.OpenFile(historyPath, os.O_WRONLY | os.O_CREATE, os.ModePerm)
	if err != nil {
		//fmt.Printf("%v\n", err)
		return false
	}

	defer f.Close()
/*
	for _, key := range keys {
		f.WriteString(key.results)
	}
 */
	ret, err := json.Marshal(keys)
	if err != nil {
		fmt.Printf("Failed to json Marshal: %v\n", err)
		return false
	}
	//fmt.Printf("ret: %s\n", ret)
	_, _ = f.Write(ret)
	fmt.Printf("saved to %s\n", historyPath)

	return true
}

// one reqeus
func requestOpengrok(files chan<- FilePrio, request_url string, sourceRoot string, workfile string) {
	//fmt.Println(request_url)
	resp, err := http.Get(request_url)
	if err != nil {
		fmt.Printf("search failed %v\n", resp)
		os.Exit(2)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Printf("%s", string(body))
	var mapResult map[string]interface{}
	err = json.Unmarshal(body, &mapResult)
	if err != nil {
		fmt.Printf("Json to map failed : %v\n", err)
		os.Exit(3)
	}


	//fmt.Printf("%v\n", mapResult)
	//searchTime, _ := mapResult["time"]
	resultCount, _ := mapResult["resultCount"]
	if resultCount != 0 {
		//keylist := make([]string, 1000)
		//var sortResult map[string]string = make(map[string]string);
		results := mapResult["results"].(map[string]interface{})

		//fmt.Println("workfile %s sourceRoot %s\n", workfile, sourceRoot)
		if sourceRoot[len(sourceRoot)-1] != '/' {
			workfile = workfile[len(sourceRoot):]
		} else {
			workfile = workfile[len(sourceRoot)-1 :]
		}

		{
			//fmt.Println(results)
			processFilePrio(files, results, workfile, sourceRoot)
		}
	}
}

func processFilePrio(files chan<- FilePrio, mapsResult map[string]interface{}, workfile string, sourceRoot string) {
	//fmt.Println(mapsResult)
	regSq := regexp.MustCompile(`<[^>]*>`)
	//regHtml := regexp.MustCompile(`(<html>)(.*)(<b>[\w.]+</b>)`)
	processor := func(files chan<- FilePrio, p string, workfile string, wg *sync.WaitGroup, sourceRoot string, lines []interface{}) {
		defer wg.Done()
		var results string
		filepath := path.Join(sourceRoot, p)
		for _, v1 := range lines {
			vmap := v1.(map[string]interface{})
			line := vmap["line"].(string)
			lineNumber := vmap["lineNumber"].(string)
			l := line
			//l, err := url.QueryUnescape(line)
			//if err != nil {
			//	l = linete
				//fmt.Println(err)
			//}
			//if strings.Contains(l, "<html>") {
			//	l = regHtml.ReplaceAllString(l, `${3}:::${2}`)
				//continue
			//}
			//l = html.UnescapeString(l)
			//l = strings.ReplaceAll(l, "<b>", "")
			//l = strings.ReplaceAll(l, "</b>", "")
			//l = strings.ReplaceAll(l, "<html>", fcontent + ":::")

			l = regSq.ReplaceAllString(l, "")
			l = strings.ReplaceAll(l,"&lt;", "<")
			l = strings.ReplaceAll(l,"&gt;", ">")
			l = strings.ReplaceAll(l, "&amp;", "&")
			l = strings.ReplaceAll(l, "\r", "")

			results += filepath + ":" + lineNumber + ": " + l + "\n"
			//fmt.Printf(results)
		}
		files <- FilePrio{p, calPrio(p, workfile), results}
	}
	var wg sync.WaitGroup
	for k := range mapsResult {
		wg.Add(1)
		go processor(files, k, workfile, &wg, sourceRoot, mapsResult[k].([]interface{}) )
	}
	wg.Wait()
	//close(files)
}

func init() {
	rootCmd.AddCommand(searchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// searchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	searchCmd.Flags().StringVarP(&configFile, "config", "R", "", "configuration file for opengrok")
	searchCmd.Flags().StringVarP(&project, "project", "p", "aosp", "project to search")
	searchCmd.Flags().StringVarP(&def, "define", "d", "", "to search define")
	searchCmd.Flags().StringVarP(&symbol, "reference", "r", "", "to search reference")
	searchCmd.Flags().StringVarP(&text, "text", "f", "", "to search text")
	//searchCmd.Flags().StringVarP(&filename, "file", "f", "", "to search file")
	searchCmd.Flags().StringVarP(&workfile, "workfile", "w", "", "work file")
}
