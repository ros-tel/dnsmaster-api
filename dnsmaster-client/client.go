package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type (
	apiClient struct {
		c     *http.Client
		token string
	}
)

var (
	err error

	token_file = flag.String("token_file", "", "Usage: -token_file=path/token_file")
	action     = flag.String("action", "", "Usage: -action=<list|add|del|commit>")
	zone       = flag.String("zone", "", "Usage: -zone=example.com")
	service    = flag.String("service", "", "Usage: -service=EXAMPLE")

	resouce_name = flag.String("resouce_name", "", "Usage: -resouce_name=srv3")
	ipv4         = flag.String("ipv4", "", "Usage: -ipv4=8.8.8.8")
	ipv6         = flag.String("ipv6", "", "Usage: -ipv4=::1")
	ttl          = flag.Int("ttl", 60, "Usage: -ttl=3600")
)

func main() {
	flag.Parse()

	ipset := false
	if *ipv4 != "" {
		ipset = true
	}
	if *ipv6 != "" {
		ipset = true
	}

	if *token_file == "" || *action == "" || *zone == "" || *service == "" {
		flag.PrintDefaults()
		log.Fatal("Unexpected or empty parameter \"token_file\" or \"action\" or \"zone\" or \"service\"")
	} else if *action == "add" && (!ipset || *resouce_name == "") {
		flag.PrintDefaults()
		log.Fatal("Unexpected or empty parameter \"resouce_name\" or \"ipv4\" or \"ipv6\"")
	} else if *action == "del" && *resouce_name == "" {
		flag.PrintDefaults()
		log.Fatal("Unexpected or empty parameter \"resouce_name\"")
	}

	token, err := getToken(*token_file)
	if err != nil {
		log.Fatal(err)
	}

	cl := &apiClient{
		c: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 5,
			},
		},
		token: token,
	}

	switch *action {
	case "commit":
		cl.commitZone()
	case "list":
		cl.listZone()
	case "add":
		if *ipv4 != "" {
			cl.addResource("A", *resouce_name, *ipv4)
		}
		if *ipv6 != "" {
			cl.addResource("AAAA", *resouce_name, *ipv6)
		}
	case "del":
		if *ipv4 != "" {
			cl.delResource("A", *resouce_name, *ipv4)
		}
		if *ipv6 != "" {
			cl.delResource("AAAA", *resouce_name, *ipv6)
		}
		if !ipset {
			// delete all "A" and "AAAA" resource
			cl.delResource("A", *resouce_name, "")
			cl.delResource("AAAA", *resouce_name, "")
		}
	default:
		flag.PrintDefaults()
		log.Fatal("Unexpected or empty parameter \"action\":", *action)
	}
}

func getToken(file_path string) (string, error) {
	f, err := os.OpenFile(file_path, os.O_RDONLY, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

func (cl *apiClient) listZone() {
	contents, err := cl.apiRequest("GET", "https://api.nic.ru/dns-master/services/"+*service+"/zones/"+*zone+"/records", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Content: %s\n", contents)
}

func (cl *apiClient) commitZone() {
	contents, err := cl.apiRequest("POST", "https://api.nic.ru/dns-master/services/"+*service+"/zones/"+*zone+"/commit", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Content: %s\n", contents)
}

func (cl *apiClient) findResouceID(rtype, rname, ip string) (string, error) {
	var resp struct {
		Status string `xml:"status"`
		Data   struct {
			Zone struct {
				Rr []struct {
					Id      string `xml:"id,attr"`
					Name    string `xml:"name"`
					IdnName string `xml:"idn-name"`
					Type    string `xml:"type"`
					A       string `xml:"a"`
					AAAA    string `xml:"aaaa"`
				} `xml:"rr"`
			} `xml:"zone"`
		} `xml:"data"`
	}

	contents, err := cl.apiRequest("GET", "https://api.nic.ru/dns-master/services/"+*service+"/zones/"+*zone+"/records", nil)
	if err != nil {
		return "", err
	}

	err = xml.Unmarshal(contents, &resp)
	if err != nil {
		log.Fatal(err)
	}

	for _, v := range resp.Data.Zone.Rr {
		if v.Type == rtype && v.Name == rname {
			if ip != "" {
				if v.A == ip || v.AAAA == ip {
					return v.Id, nil
				}
				continue
			} else {
				return v.Id, nil
			}
		}
	}

	return "", nil
}

func (cl *apiClient) addResource(rtype, rname, ip string) error {
	rid, err := cl.findResouceID(rtype, rname, ip)
	if err != nil {
		return err
	}

	if rid != "" {
		log.Printf("Resource name \"%s\" type \"%s\" exists id \"%s\"", rname, rtype, rid)
		return nil
	}

	type Rr struct {
		XMLName xml.Name `xml:"request"`
		Name    string   `xml:"rr-list>rr>name"`
		Ttl     int      `xml:"rr-list>rr>ttl"`
		Type    string   `xml:"rr-list>rr>type"`
		A       string   `xml:"rr-list>rr>a,omitempty"`
		AAAA    string   `xml:"rr-list>rr>aaaa,omitempty"`
	}
	r := &Rr{
		Ttl:  *ttl,
		Type: rtype,
		Name: rname + "." + *zone + ".",
	}

	if rtype == "A" {
		r.A = ip
	} else if rtype == "AAAA" {
		r.AAAA = ip
	} else {
		return fmt.Errorf("Adding a resource of type \"%s\" is not implemented", rtype)
	}

	rdata, err := xml.Marshal(r)
	if err != nil {
		log.Fatal(err)
	}

	contents, err := cl.apiRequest("PUT", "https://api.nic.ru/dns-master/services/"+*service+"/zones/"+*zone+"/records", bytes.NewReader(append([]byte(xml.Header)[:], rdata[:]...)))
	if err != nil {
		return err
	}
	fmt.Printf("Content: %s\n", contents)

	return nil
}

func (cl *apiClient) delResource(rtype, rname, ip string) error {
	for {
		rid, err := cl.findResouceID(rtype, rname, ip)
		if err != nil {
			return err
		}

		// Resource not exists
		if rid == "" {
			return nil
		}

		contents, err := cl.apiRequest("DELETE", "https://api.nic.ru/dns-master/services/"+*service+"/zones/"+*zone+"/records/"+rid, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Content: %s\n", contents)
	}
	return nil
}

func (cl *apiClient) apiRequest(method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cl.token)
	resp, err := cl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return contents, nil
}
