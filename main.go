package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

type Secrets struct {
	Key    string `json:"apikey"`
	Secret string `json:"secretapikey"`
}

type Record struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	RecordType string `json:"type"`
	Content    string `json:"content"`
	TTL        string `json:"ttl"`
}

type IPResp struct {
	Status string `json:"status"`
	IP     string `json:"yourIp"`
}

type RecordsResp struct {
	Status  string   `json:"status"`
	Records []Record `json:"records"`
}

type EditRecordReq struct {
	Key        string `json:"apikey"`
	Secret     string `json:"secretapikey"`
	Name       string `json:"name"`
	RecordType string `json:"type"`
	Content    string `json:"content"`
	TTL        string `json:"ttl"`
}

var secrets Secrets
var BASE_PATH = "https://porkbun.com/api/json/v3"
var filepath = os.ExpandEnv("$HOME") + "/.pddc"
var domain string
var currentIP string

func init() {
	loadSecrets()
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalln("Unable to parse domain argument")
	}

	domain := os.Args[1]

	if domain == "ping" {
		ip, err := getIP()
		if err != nil {
			log.Fatalln("ping failed: ", err)
		}
		fmt.Printf("your ip is %s", ip)
	} else {
		ip, err := getIP()
		if err != nil {
			fmt.Println(err)
		}

		prevIP, err := getPrevIP()
		if err != nil {
			fmt.Println(err)
		}

		if ip != prevIP {
			tempRecords := fetchRecords()
			records := filterRecords(ip, tempRecords)
			if len(records) > 0 {
				updateRecords(records)
			}
		}
	}

}

func fetchRecords() []Record {
	path := "/dns/retrieve/" + domain
	jsonBody, err := json.Marshal(secrets)
	if err != nil {
		fmt.Println(err)
	}
	resp := postRequest(path, jsonBody)
	var rec []Record

	if resp.StatusCode == http.StatusOK {
		var jsonResp RecordsResp
		json.NewDecoder(resp.Body).Decode(&jsonResp)
		return jsonResp.Records
	} else {
		log.Fatalf("Could not fetch records, status %d", resp.StatusCode)
		return rec
	}
}

func getIP() (string, error) {
	jsonBody, err := json.Marshal(secrets)
	if err != nil {
		fmt.Println(err)
	}
	resp := postRequest("/ping", jsonBody)

	if resp.StatusCode == http.StatusOK {
		var jsonResp IPResp
		json.NewDecoder(resp.Body).Decode(&jsonResp)
		currentIP = jsonResp.IP
		return jsonResp.IP, nil
	} else {
		return "", errors.New(fmt.Sprintf("API request returned status: %d", resp.StatusCode))
	}
}

func updateRecords(records []Record) {
	for _, rec := range records {
		updateRecord(rec)
	}
	updateIP(currentIP)
}

func createEditRecordReq(record Record) EditRecordReq {
	var eRec EditRecordReq
	eRec.Key = secrets.Key
	eRec.Secret = secrets.Secret
	eRec.Content = record.Content
	eRec.RecordType = record.RecordType
	eRec.Name = record.Name
	eRec.TTL = record.TTL

	return eRec
}

func updateRecord(record Record) {
	rec := createEditRecordReq(record)
	jsonBody, err := json.Marshal(rec)
	if err != nil {
		fmt.Println(err)
	}
	path := "/dns/edit/" + domain + "/" + record.ID
	resp := postRequest(path, jsonBody)

	if resp.StatusCode == http.StatusOK {
		log.Printf("Record updated for %v", record)
	} else {
		log.Printf("Unable to update record for %v, status %v", record, resp.StatusCode)
	}

}

func filterRecords(ip string, records []Record) []Record {
	var fr []Record
	for _, v := range records {
		if strings.Contains(v.Name, domain) && v.RecordType == "A" && v.Content != ip {
			v.Content = ip
			if v.Name == domain {
				v.Name = ""
			} else {
				v.Name = strings.Split(v.Name, ".")[0]
			}
			fr = append(fr, v)
		}
	}

	return fr
}

func loadSecrets() {
	bytes := readJSONFile("secrets.json")
	json.Unmarshal(bytes, &secrets)
}

func readJSONFile(filename string) []byte {
	jsonFile, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()
	bytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
	}

	return bytes
}

func readJSONFileToStruct(filename string, s *interface{}) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()
	bytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
	}

	json.Unmarshal(bytes, &s)
}

func updateIP(ip string) {
	prevIP, _ := getPrevIP()

	if prevIP != ip {
		setPrevIP(ip)
	}
}

func getPrevIP() (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err

	}
	defer file.Close()

	var ip string
	decoder := gob.NewDecoder(file)
	decoder.Decode(&ip)

	return ip, nil
}

func setPrevIP(ip string) {
	f, err := os.Create(filepath)
	if err != nil {
		log.Fatalln("Error opening/creating file: ", err)
	}
	encoder := gob.NewEncoder(f)

	err = encoder.Encode(ip)
	if err != nil {
		log.Fatalln("Error encoding gob: ", err)
	}

	f.Close()
}

func postRequest(path string, body []byte) *http.Response {
	client := &http.Client{}
	req, err := http.NewRequest("POST", BASE_PATH+path, bytes.NewReader(body))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	return resp
}
