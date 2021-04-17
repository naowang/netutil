// netutil project netutil.go
package netutil

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func UnGzip(data []byte) ([]byte, error) {
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, data)
	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	} else {
		defer r.Close()
		undatas, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}
		return undatas, nil
	}
}

func UnDeflate(data []byte) ([]byte, error) {
	b := new(bytes.Buffer)
	binary.Write(b, binary.LittleEndian, data)
	r := flate.NewReader(b)
	defer r.Close()
	undatas, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return undatas, nil
}

func UncompressWithName(data []byte, name string) ([]byte, error) {
	if name == "gzip" {
		return UnGzip(data)
	} else if name == "deflate" {
		return UnDeflate(data)
	}
	err := errors.New("unknow compress method")
	return []byte(""), err
}

//httpgetdata format name follow value sequence.
func UrlGet(httpurl string, httpgetdata []string, onlyhead bool, httpsendhead []string, cookie []*http.Cookie, contimeout, datatrantimeout time.Duration, outbuf []byte, getctt_contenttype_regex ...string) (content []byte, head http.Header, retcookie []*http.Cookie, httpretcode int, redilocation string) {
	if len(httpgetdata)%2 != 0 {
		return []byte(""), http.Header{}, nil, 3, redilocation
	}
	if contimeout <= 0 {
		contimeout = 36500 * 24 * 3600 * time.Second
	}
	if datatrantimeout <= 0 {
		datatrantimeout = 36500 * 24 * 3600 * time.Second
	}

	urlparam := url.Values{}
	getdatai := 0
	for getdatai < len(httpgetdata) {
		urlparam.Set(httpgetdata[getdatai], httpgetdata[getdatai+1])
		getdatai += 2
		if getdatai >= len(httpgetdata) {
			break
		}
	}
	urlparamstr := urlparam.Encode()
	if len(urlparamstr) > 0 {
		if strings.Index(httpurl, "?") == -1 {
			httpurl += "?" + urlparamstr
		} else {
			httpurl += "&" + urlparamstr
		}
	}

	client := &http.Client{Transport: &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, contimeout) //设置建立连接超时
			if err != nil {
				return nil, err
			}
			c.SetDeadline(time.Now().Add(datatrantimeout)) //设置发送接收数据超时
			return c, nil
		},
	},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		redilocation = req.URL.String()
		return nil
	}
	request, err := http.NewRequest("GET", httpurl, nil)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 1, redilocation
	}
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	if cookie != nil {
		for _, ck := range cookie {
			request.AddCookie(ck) //request中添加cookie
		}
	}

	//设置request的header
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpsendheadi := 0
	haveacceptencoding := false
	for httpsendheadi < len(httpsendhead) {
		if httpsendhead[httpsendheadi] == "Accept-Encoding" {
			haveacceptencoding = true
		}
		if httpsendhead[httpsendheadi+1] != "" {
			request.Header.Set(httpsendhead[httpsendheadi], httpsendhead[httpsendheadi+1])
		}
		httpsendheadi += 2
		if httpsendheadi >= len(httpsendhead) {
			break
		}
	}
	if haveacceptencoding == false {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}
	//fmt.Println("UrlGet client.Do(request) 1")
	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 2, redilocation
	}
	//fmt.Println("UrlGet client.Do(request) 1 end")

	defer response.Body.Close()
	if response.StatusCode == response.StatusCode {
		//fmt.Println(reflect.TypeOf(response.Header))
		//fmt.Println(response.Header)
		//head, err := ioutil.ReadFile(response.Header)
		if onlyhead {
			return []byte(""), response.Header, response.Cookies(), response.StatusCode, redilocation
		} else {
			if len(getctt_contenttype_regex) > 0 {
				contenttype := response.Header.Get("Content-Type")
				if !regexp.MustCompile(getctt_contenttype_regex[0]).MatchString(contenttype) {
					return []byte(""), response.Header, response.Cookies(), response.StatusCode, redilocation
				}
			}
			var data []byte
			var wcnt int
			if outbuf != nil {
				wcnt, err = io.ReadFull(response.Body, outbuf)
				if err == nil || err.Error() == "unexpected EOF" {
					data = outbuf[:wcnt]
					err = nil
				}
			} else {
				data, err = ioutil.ReadAll(response.Body)
			}
			if err == nil {
				encodeingname := response.Header.Get("Content-Encoding")
				if encodeingname == "" {
					return data, response.Header, response.Cookies(), response.StatusCode, redilocation
				} else {
					undata, err := UncompressWithName(data, strings.ToLower(encodeingname))
					if err == nil {
						return undata, response.Header, response.Cookies(), response.StatusCode, redilocation
					} else {
						return []byte(""), response.Header, response.Cookies(), 5, redilocation
					}
				}
			}
		}
	}
	return []byte(""), http.Header{}, nil, response.StatusCode, redilocation
}

func UrlPost(httpurl string, postdata []string, onlyhead bool, httpsendhead []string, cookie []*http.Cookie, contimeout, datatrantimeout time.Duration) (content []byte, head http.Header, retcookie []*http.Cookie, httpretcode int, redilocation string) {
	if len(postdata)%2 != 0 {
		return []byte(""), http.Header{}, nil, 3, redilocation
	}
	if contimeout <= 0 {
		contimeout = 36500 * 24 * 3600 * time.Second
	}
	if datatrantimeout <= 0 {
		datatrantimeout = 36500 * 24 * 3600 * time.Second
	}

	urlparam := url.Values{}
	postdatai := 0
	for postdatai < len(postdata) {
		urlparam.Set(postdata[postdatai], postdata[postdatai+1])
		postdatai += 2
		if postdatai >= len(postdata) {
			break
		}
	}
	urlparamstr := urlparam.Encode()

	client := &http.Client{Transport: &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, contimeout) //设置建立连接超时
			if err != nil {
				return nil, err
			}
			c.SetDeadline(time.Now().Add(datatrantimeout)) //设置发送接收数据超时
			return c, nil
		},
	},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		redilocation = req.URL.String()
		return nil
	}

	request, err := http.NewRequest("POST",
		httpurl,
		strings.NewReader(urlparamstr))

	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 1, redilocation
	}
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	if cookie != nil {
		for _, ck := range cookie {
			request.AddCookie(ck) //request中添加cookie
		}
	}

	//设置request的header
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpsendheadi := 0
	haveacceptencoding := false
	for httpsendheadi < len(httpsendhead) {
		if httpsendhead[httpsendheadi] == "Accept-Encoding" {
			haveacceptencoding = true
		}
		if httpsendhead[httpsendheadi+1] != "" {
			request.Header.Set(httpsendhead[httpsendheadi], httpsendhead[httpsendheadi+1])
		}
		httpsendheadi += 2
		if httpsendheadi >= len(httpsendhead) {
			break
		}
	}
	if haveacceptencoding == false {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 2, redilocation
	}

	defer response.Body.Close()
	if response.StatusCode == response.StatusCode {
		//fmt.Println(reflect.TypeOf(response.Header))
		//fmt.Println(response.Header)
		//head, err := ioutil.ReadFile(response.Header)
		if onlyhead {
			return []byte(""), response.Header, response.Cookies(), response.StatusCode, redilocation
		} else {
			data, err := ioutil.ReadAll(response.Body)
			if err == nil {
				if response.Header.Get("Content-Encoding") == "" {
					return data, response.Header, response.Cookies(), response.StatusCode, redilocation
				} else {
					undata, err := UncompressWithName(data, strings.ToLower(response.Header.Get("Content-Encoding")))
					if err == nil {
						return undata, response.Header, response.Cookies(), response.StatusCode, redilocation
					} else {
						return []byte(""), response.Header, response.Cookies(), 5, redilocation
					}
				}
			}
		}
	}
	return []byte(""), http.Header{}, nil, response.StatusCode, redilocation
}

//multi file upload in one segment need add postfield name with "[]"
func UrlPostWithFile(httpurl string, postdata []string, onlyhead bool, httpsendhead []string, cookie []*http.Cookie, contimeout, datatrantimeout time.Duration) (content []byte, head http.Header, retcookie []*http.Cookie, httpretcode int, redilocation string) {
	if len(postdata)%2 != 0 {
		return []byte(""), http.Header{}, nil, 3, redilocation
	}
	if contimeout <= 0 {
		contimeout = 36500 * 24 * 3600 * time.Second
	}
	if datatrantimeout <= 0 {
		datatrantimeout = 36500 * 24 * 3600 * time.Second
	}

	client := &http.Client{Transport: &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, contimeout) //设置建立连接超时
			if err != nil {
				return nil, err
			}
			c.SetDeadline(time.Now().Add(datatrantimeout)) //设置发送接收数据超时
			return c, nil
		},
	},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		redilocation = req.URL.String()
		return nil
	}

	// Create buffer
	buf := &bytes.Buffer{} // caveat IMO dont use this for large files, \
	// create a tmpfile and assemble your multipart from there (not tested)
	w := multipart.NewWriter(buf)
	postdatai := 0
	for postdatai < len(postdata) {
		postfi, err := os.Stat(postdata[postdatai+1])
		if err == nil && !postfi.IsDir() {
			// Create file field
			fw, err := w.CreateFormFile(postdata[postdatai], filepath.Base(postdata[postdatai+1])) //这里的file很重要，必须和服务器端的FormFile一致
			if err != nil {
				//fmt.Println("c")
				return []byte(""), http.Header{}, nil, 10, redilocation
			}
			fd, err := os.Open(postdata[postdatai+1])
			if err != nil {
				//fmt.Println("d")
				return []byte(""), http.Header{}, nil, 11, redilocation
			}
			defer fd.Close()
			// Write file field from file to upload
			_, err = io.Copy(fw, fd)
			if err != nil {
				//fmt.Println("e")
				return []byte(""), http.Header{}, nil, 12, redilocation
			}

		} else {
			//other post data
			w.WriteField(postdata[postdatai], postdata[postdatai+1])
		}

		postdatai += 2
		if postdatai >= len(postdata) {
			break
		}
	}
	// Important if you do not close the multipart writer you will not have a
	// terminating boundry
	w.Close()

	request, err := http.NewRequest("POST",
		httpurl,
		buf)

	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 1, redilocation
	}

	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	if cookie != nil {
		for _, ck := range cookie {
			request.AddCookie(ck) //request中添加cookie
		}
	}

	//设置request的header
	request.Header.Set("Content-Type", w.FormDataContentType())
	httpsendheadi := 0
	haveacceptencoding := false
	for httpsendheadi < len(httpsendhead) {
		if httpsendhead[httpsendheadi] == "Accept-Encoding" {
			haveacceptencoding = true
		}
		if httpsendhead[httpsendheadi+1] != "" {
			request.Header.Set(httpsendhead[httpsendheadi], httpsendhead[httpsendheadi+1])
		}
		httpsendheadi += 2
		if httpsendheadi >= len(httpsendhead) {
			break
		}
	}
	if haveacceptencoding == false {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 2, redilocation
	}

	defer response.Body.Close()
	if response.StatusCode == response.StatusCode {
		//fmt.Println(reflect.TypeOf(response.Header))
		//fmt.Println(response.Header)
		//head, err := ioutil.ReadFile(response.Header)
		if onlyhead {
			return []byte(""), response.Header, response.Cookies(), response.StatusCode, redilocation
		} else {
			data, err := ioutil.ReadAll(response.Body)
			if err == nil {
				if response.Header.Get("Content-Encoding") == "" {
					return data, response.Header, response.Cookies(), response.StatusCode, redilocation
				} else {
					undata, err := UncompressWithName(data, strings.ToLower(response.Header.Get("Content-Encoding")))
					if err == nil {
						return undata, response.Header, response.Cookies(), response.StatusCode, redilocation
					} else {
						return []byte(""), response.Header, nil, 5, redilocation
					}
				}
			}
		}
	}
	return []byte(""), http.Header{}, nil, response.StatusCode, redilocation
}

func UrlGetToFile(httpurl string, httpgetdata []string, onlyhead bool, httpsendhead []string, cookie []*http.Cookie, filepath string, contimeout, datatrantimeout time.Duration) (head http.Header, retcookie []*http.Cookie, httpretcode int, redilocation string) {
	if len(httpgetdata)%2 != 0 {
		return http.Header{}, nil, 3, redilocation
	}
	if contimeout <= 0 {
		contimeout = 36500 * 24 * 3600 * time.Second
	}
	if datatrantimeout <= 0 {
		datatrantimeout = 36500 * 24 * 3600 * time.Second
	}

	urlparam := url.Values{}
	getdatai := 0
	for getdatai < len(httpgetdata) {
		urlparam.Set(httpgetdata[getdatai], httpgetdata[getdatai+1])
		getdatai += 2
		if getdatai >= len(httpgetdata) {
			break
		}
	}
	urlparamstr := urlparam.Encode()
	if len(urlparamstr) > 0 {
		if strings.Index(httpurl, "?") == -1 {
			httpurl += "?" + urlparamstr
		} else {
			httpurl += "&" + urlparamstr
		}
	}

	client := &http.Client{Transport: &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, contimeout) //设置建立连接超时
			if err != nil {
				return nil, err
			}
			c.SetDeadline(time.Now().Add(datatrantimeout)) //设置发送接收数据超时
			return c, nil
		},
	},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		redilocation = req.URL.String()
		return nil
	}

	request, err := http.NewRequest("GET", httpurl, nil)
	if err != nil {
		fmt.Println(err)
		return http.Header{}, nil, 1, redilocation
	}
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	//request.AddCookie(cookie) //request中添加cookie
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	if cookie != nil {
		for _, ck := range cookie {
			request.AddCookie(ck) //request中添加cookie
		}
	}

	//设置request的header
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	//request.Header.Set("Range", "bytes="+strconv.FormatInt(stat.Size(), 10)+"-")
	httpsendheadi := 0
	haveacceptencoding := false
	for httpsendheadi < len(httpsendhead) {
		if httpsendhead[httpsendheadi] == "Accept-Encoding" {
			haveacceptencoding = true
		}
		if httpsendhead[httpsendheadi+1] != "" {
			request.Header.Set(httpsendhead[httpsendheadi], httpsendhead[httpsendheadi+1])
		}
		httpsendheadi += 2
		if httpsendheadi >= len(httpsendhead) {
			break
		}
	}
	if haveacceptencoding == false {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return http.Header{}, nil, 2, redilocation
	}

	defer response.Body.Close()
	if response.StatusCode == response.StatusCode {
		//fmt.Println(reflect.TypeOf(response.Header))
		//fmt.Println(response.Header)
		//head, err := ioutil.ReadFile(response.Header)
		if onlyhead {
			return response.Header, response.Cookies(), response.StatusCode, redilocation
		} else {
			if response.Header.Get("Content-Encoding") == "" {
				f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
				if err != nil {
					return response.Header, response.Cookies(), 4, redilocation
				}
				defer f.Close()
				io.Copy(f, response.Body)
				return response.Header, response.Cookies(), response.StatusCode, redilocation
			} else {
				data, err := ioutil.ReadAll(response.Body)
				undata, err := UncompressWithName(data, strings.ToLower(response.Header.Get("Content-Encoding")))
				if err == nil {
					f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0666)
					if err != nil {
						return response.Header, response.Cookies(), 6, redilocation
					}
					defer f.Close()
					f.Write(undata)
				} else {
					return response.Header, response.Cookies(), 5, redilocation
				}
			}
		}
	}
	return http.Header{}, nil, response.StatusCode, redilocation
}

func UrlGetWithRange(httpurl string, httpgetdata []string, onlyhead bool, httpsendhead []string, cookie []*http.Cookie, startpos, endpos int64, contimeout, datatrantimeout time.Duration) (content []byte, head http.Header, retcookie []*http.Cookie, httpretcode int, redilocation string) {
	if len(httpgetdata)%2 != 0 {
		return []byte(""), http.Header{}, nil, 3, redilocation
	}
	if contimeout <= 0 {
		contimeout = 36500 * 24 * 3600 * time.Second
	}
	if datatrantimeout <= 0 {
		datatrantimeout = 36500 * 24 * 3600 * time.Second
	}

	urlparam := url.Values{}
	getdatai := 0
	for getdatai < len(httpgetdata) {
		urlparam.Set(httpgetdata[getdatai], httpgetdata[getdatai+1])
		getdatai += 2
		if getdatai >= len(httpgetdata) {
			break
		}
	}
	client := &http.Client{Transport: &http.Transport{
		Dial: func(netw, addr string) (net.Conn, error) {
			c, err := net.DialTimeout(netw, addr, contimeout) //设置建立连接超时
			if err != nil {
				return nil, err
			}
			c.SetDeadline(time.Now().Add(datatrantimeout)) //设置发送接收数据超时
			return c, nil
		},
	},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		redilocation = req.URL.String()
		return nil
	}
	request, err := http.NewRequest("GET", httpurl, nil)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 1, redilocation
	}
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	//request.AddCookie(cookie) //request中添加cookie
	//cookie := &http.Cookie{Name: "userId", Value: strconv.Itoa(12345)}
	if cookie != nil {
		for _, ck := range cookie {
			request.AddCookie(ck) //request中添加cookie
		}
	}

	//设置request的header
	request.Header.Set("Content-Encoding", "application/x-www-form-urlencoded")
	request.Header.Set("Range", "bytes="+strconv.FormatInt(startpos, 10)+"-"+strconv.FormatInt(endpos, 10))
	httpsendheadi := 0
	haveacceptencoding := false
	for httpsendheadi < len(httpsendhead) {
		if httpsendhead[httpsendheadi] == "Accept-Encoding" {
			haveacceptencoding = true
		}
		if httpsendhead[httpsendheadi+1] != "" {
			request.Header.Set(httpsendhead[httpsendheadi], httpsendhead[httpsendheadi+1])
		}
		httpsendheadi += 2
		if httpsendheadi >= len(httpsendhead) {
			break
		}
	}
	if haveacceptencoding == false {
		request.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
		return []byte(""), http.Header{}, nil, 2, redilocation
	}

	defer response.Body.Close()
	if response.StatusCode == response.StatusCode {
		//fmt.Println(reflect.TypeOf(response.Header))
		//fmt.Println(response.Header)
		//head, err := ioutil.ReadFile(response.Header)
		if onlyhead {
			return []byte(""), response.Header, response.Cookies(), response.StatusCode, redilocation
		} else {
			//fmt.Println("range to here")
			data, err := ioutil.ReadAll(response.Body)
			if err == nil {
				if response.Header.Get("Content-Encoding") == "" {
					return data, response.Header, response.Cookies(), response.StatusCode, redilocation
				} else {
					undata, err := UncompressWithName(data, strings.ToLower(response.Header.Get("Content-Encoding")))
					if err == nil {
						return undata, response.Header, response.Cookies(), response.StatusCode, redilocation
					} else {
						return []byte(""), response.Header, response.Cookies(), 5, redilocation
					}
				}
			}
		}
	}
	return []byte(""), http.Header{}, nil, response.StatusCode, redilocation
}

func UrlDecode(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '%' {
			val, vale := strconv.ParseInt(name[i : i+3][1:], 16, 32)
			if vale == nil {
				name = name[:i] + string([]byte{byte(val)}) + name[i+3:]
			}
		}
	}
	return name
}

func UrlEncode(namedata string) string {
	var firstslashpos int
	if strings.Index(namedata[8:], "/") != -1 {
		firstslashpos = 8 + strings.Index(namedata[8:], "/")
	}
	for i := len(namedata) - 1; i >= 0; i-- {
		if i >= firstslashpos && (namedata[i] >= 0 && namedata[i] <= 32 || namedata[i] >= 128 || namedata[i] == '\\' || namedata[i] == '"' || namedata[i] == '$' || namedata[i] == '#' || namedata[i] == '^' || namedata[i] == '{' || namedata[i] == '}') {
			namedata = namedata[:i] + "%" + fmt.Sprintf("%02x", namedata[i]) + namedata[i+1:]
		}
	}
	return namedata
}
