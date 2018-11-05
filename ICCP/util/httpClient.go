package util

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func HttpClient(data, url, logFileName string) error {
	//生成client 参数为默认
	client := &http.Client{}

	//提交请求
	reqest, err := http.NewRequest("POST", url, strings.NewReader(data))
	reqest.Close = true
	reqest.Header.Set("Content-Type", "application/json")
	//reqest.Header.Set("Authorization", "qwertyuiopasdfghjklzxcvbnm1234567890")

	if err != nil {
		panic(err)
	}

	//处理返回结果
	response, err := client.Do(reqest)
	if err != nil {
		log.Fatal("response: %s", err)

	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal("responseBody: %s", err)
	}
	responseBodyStr := string(responseBody)

	defer response.Body.Close()

	//将异常请求报文写入日志文件
	if !strings.Contains(responseBodyStr, "\"result\":\"0000\"") {

		var requestJsonBuffer bytes.Buffer
		requestJsonBuffer.WriteString(time.Now().Format("2006-01-02 15:04:05"))
		requestJsonBuffer.WriteString(" TOMCAT ")
		requestJsonBuffer.WriteString(responseBodyStr)
		requestJsonBuffer.WriteString(" --- GOOUT ")
		requestJsonBuffer.WriteString(data)
		go AppendToFile(logFileName, requestJsonBuffer.String())
	}

	//fmt.Println("mac=", responseBodyStr)
	//返回的状态码
	//status := response.StatusCode

	//fmt.Println(status)

	return err
}
