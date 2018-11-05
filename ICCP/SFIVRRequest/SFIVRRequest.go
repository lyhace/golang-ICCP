package SFIVRRequest

import (
	"ICCP/util"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	uriSFIVRRequest          = flag.String("uriSFIVRRequest", "amqp://iccprequest:iccprequest@iccpmq:5672/iccprequest", "AMQP URI")
	exchangeNameSFIVRRequest = flag.String("exchangeSFIVRRequest", "iccprequest", "Durable AMQP exchange name")
	exchangeTypeSFIVRRequest = flag.String("exchange-typeSFIVRRequest", "direct", "Exchange type - direct|fanout|topic|x-custom")
	routingKeySFIVRRequest   = flag.String("keySFIVRRequest", "SFIVRRequest", "AMQP routing key")
	reliableSFIVRRequest     = flag.Bool("reliableSFIVRRequest", true, "Wait for the publisher confirmation before exiting")
	//请求日志
	logFileName = "/home/smp/logs/SFIVRPublish_" + time.Now().Format("2006-01-02") + ".log"
	//logFileName = "SFIVRPublish_" + time.Now().Format("2006-01-02") + ".log"
	//异常响应日志
	logFileNameError = "/home/smp/logs/SFIVRResponseError_" + time.Now().Format("2006-01-02") + ".log"
	//logFileNameResponse = "SFIVRResponseError_" + time.Now().Format("2006-01-02") + ".log"
)

func init() {
	flag.Parse()
}

//处理错误函数
func checkErrs(err error) {
	if err != nil {
		panic(err)
		log.Println("Read failed:", err)
	}
}

type RequestJson struct {
	Header   RequestHeader `json:"header"`
	Body	RequestBody   `json:"body"`
}

type RequestHeader struct {
	MessageId   string `json:"messageId"`
	ServiceName string `json:"serviceName"`
}

type RequestBody struct {
	MediaContent string `json:"mediaContent"`
	NumCode      string `json:"numCode"`
	DisplayNum   string `json:"displayNum"`
	CalledNum    string `json:"calledNum"`
	RequestTime    string `json:"requestTime"`
}

type ResponseJson struct {
	Header   ResponseHeader `json:"header"`
	Body	 ResponseBody   `json:"body"`
}

type ResponseHeader struct {
	MessageId   string `json:"messageId"`
	ServiceName string `json:"serviceName"`
}

type ResponseBody struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}

func RequestMQ(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		b, err := ioutil.ReadAll(r.Body)
		checkErrs(err)
		defer r.Body.Close()

		requestJson := &RequestJson{}
		err = json.Unmarshal(b, requestJson)
		if err != nil {
			log.Println("json format error: ", err)
		}

		requestJsonStrObj, requestJsonErr := json.Marshal(requestJson)
		checkErrs(requestJsonErr)
		// 二次校验剔除请求时间参数
		requestJsonStr := strings.Replace(string(requestJsonStrObj), ",\"requestTime\":\"\"", "", -1)
		requestJsonStr = strings.Replace(requestJsonStr, "\"requestTime\":\"\",", "", -1)
		log.Println("requestJsonStr: ", requestJsonStr)

		//将请求报文写入日志文件
		var requestJsonBuffer bytes.Buffer
		requestJsonBuffer.WriteString(time.Now().Format("2006-01-02 15:04:05"))
		requestJsonBuffer.WriteString(" ")
		requestJsonBuffer.WriteString(requestJsonStr)
		go util.AppendToFile(logFileName, requestJsonBuffer.String())

		var responseJson ResponseJson
		responseJson.Body.Result = "0000"
		responseJson.Body.Reason = "succ"
		responseJson.Header.MessageId = requestJson.Header.MessageId
		responseJson.Header.ServiceName = "IVRResponse"

		// 获取URL里携带的mac值
		var sfmac = ""
		var cqtmac = ""
		queryForm, err := url.ParseQuery(r.URL.RawQuery)
		if err == nil && len(queryForm["mac"]) > 0 {
			if queryForm["mac"][0] != "cqt1234" {
				//除去空格和制表符等
				reg := regexp.MustCompile("\\s*|\t|\r|\n")
				requestJsonStr = reg.ReplaceAllString(requestJsonStr, "")
				//fmt.Println("requestJsonStr=", requestJsonStr)
				// 校验mac值是否正确
				firstHash := sha256.New()
				firstHash.Write([]byte(requestJsonStr))
				firstHashStr := hex.EncodeToString(firstHash.Sum(nil))
				firstHashStr += "3F6258C8ADB64B463C31D1A826CBDBF4"
				secondHash := sha256.New()
				secondHash.Write([]byte(firstHashStr))
				secondHashStr := hex.EncodeToString(secondHash.Sum(nil))

				if secondHashStr != queryForm["mac"][0] {
					responseJson.Body.Result = "0001"
					responseJson.Body.Reason = "mac is error"
					sfmac = queryForm["mac"][0]
					cqtmac = secondHashStr
				}
				fmt.Println("getmac=", queryForm["mac"][0])
			}

		} else {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "mac is error"
		}

		//判断serviceName是否正确
		if requestJson.Header.ServiceName != "SFIVRRequest" {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "serviceName is error : " + requestJson.Header.ServiceName
		}

		//判断messageId是否为空
		if !(len(requestJson.Header.MessageId) > 0) {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "messageId Is Null!"
		}

		//判断DisplayNum是否为空
		if !(len(requestJson.Body.DisplayNum) > 0) {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "displayNumber Is Null!"
		} else {
			//判断DisplayNum是否为数字
			_, error := strconv.Atoi(requestJson.Body.DisplayNum)
			if error != nil {
				responseJson.Body.Result = "0001"
				responseJson.Body.Reason = "displayNum is error : " + requestJson.Body.DisplayNum
			}
		}

		//判断mediaContent是否为空
		if !(len(requestJson.Body.MediaContent) > 0) {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "mediaContent Is Null!"
		}

		//判断numCode是否为空
		if !(len(requestJson.Body.NumCode) > 0) {
			responseJson.Body.Result = "0001"
			responseJson.Body.Reason = "numCode Is Null!"
		} else {
			//判断numCode是否为数字
			_, error := strconv.Atoi(requestJson.Body.NumCode)
			if error != nil {
				responseJson.Body.Result = "0001"
				responseJson.Body.Reason = "numCode must be a number : " + requestJson.Body.NumCode
			}
		}

		if responseJson.Body.Result == "0000" {
			requestJson.Body.RequestTime = time.Now().Format("20060102150405")
			newRequestJsonStr, newRuestJsonErr := json.Marshal(requestJson)
			checkErrs(newRuestJsonErr)
			if err := publish(*uriSFIVRRequest, *exchangeNameSFIVRRequest, *exchangeTypeSFIVRRequest, *routingKeySFIVRRequest, string(newRequestJsonStr), *reliableSFIVRRequest); err != nil {
				responseJson.Body.Result = "9999"
				responseJson.Body.Reason = "system error"
				if strings.Contains(err.Error(), "i/o timeout") {
					responseJson.Body.Reason = "mq connection timeout"
				}
				log.Println("rabbitmq publish: ", err)
			}
		}

		responseJsonStr, responseJsonErr := json.Marshal(responseJson)
		checkErrs(responseJsonErr)

		//将请求异常报文写入日志文件
		if responseJson.Body.Result != "0000" {
			var requestJsonBufferResponse bytes.Buffer
			requestJsonBufferResponse.WriteString(time.Now().Format("2006-01-02 15:04:05"))
			requestJsonBufferResponse.WriteString(" OUT ")
			requestJsonBufferResponse.WriteString(string(responseJsonStr) + " --- SFIN " + string(b))
			if(sfmac != ""){
				requestJsonBufferResponse.WriteString(" --- SFMAC:")
				requestJsonBufferResponse.WriteString(sfmac)
			}
			if(sfmac != ""){
				requestJsonBufferResponse.WriteString(" --- CQTMAC:")
				requestJsonBufferResponse.WriteString(cqtmac)
			}
			go util.AppendToFile(logFileNameError, requestJsonBufferResponse.String())
		}

		log.Println("responseJson: ", string(responseJsonStr))
		fmt.Fprintf(w, string(responseJsonStr))
	} else {

		log.Println("Only support Post")
		fmt.Fprintf(w, "Only support post")
	}

}

func publish(amqpURI, exchange, exchangeType, routingKey, producerMsg string, reliable bool) error {

	// This function dials, connects, declares, publishes, and tears down,
	// all in one go. In a real service, you probably want to maintain a
	// long-lived connection as state, and publish against that.

	log.Println("dialing: ", amqpURI)
	//connection, err := amqp.Dial(amqpURI)
	//if err != nil {
	//	return fmt.Errorf("Dial: %s", err)
	//}

	connection, err := amqp.DialConfig(amqpURI, amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 3*time.Second)
		},
	})
	if err != nil {
		return fmt.Errorf("Dial: %s", err)
	}
	defer connection.Close()

	log.Println("got Connection, getting Channel")
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("Channel: ", err)
	}

	log.Println("got Channel, declaring ", exchangeType, " Exchange (", exchange, ")")
	if err := channel.ExchangeDeclare(
		exchange,     // name
		exchangeType, // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return fmt.Errorf("Exchange Declare: ", err)
	}

	// Reliable publisher confirms require confirm.select support from the
	// connection.
	if reliable {
		log.Println("enabling publishing confirms.")
		if err := channel.Confirm(false); err != nil {
			return fmt.Errorf("Channel could not be put into confirm mode: ", err)
		}

		confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))

		defer confirmOne(confirms)
	}

	log.Println("declared Exchange, publishing ", len(producerMsg), "B body (", producerMsg, ")")
	if err = channel.Publish(
		exchange,   // publish to an exchange
		routingKey, // routing to 0 or more queues
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			Headers:         amqp.Table{},
			ContentType:     "text/plain",
			ContentEncoding: "",
			Body:            []byte(producerMsg),
			DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
			Priority:        0,              // 0-9
			// a bunch of application/implementation-specific fields
		},
	); err != nil {
		return fmt.Errorf("Exchange Publish: ", err)
	}

	return nil
}

// One would typically keep a channel of publishings, a sequence number, and a
// set of unacknowledged sequence numbers and loop until the publishing channel
// is closed.
func confirmOne(confirms <-chan amqp.Confirmation) {
	log.Println("waiting for confirmation of one publishing")

	if confirmed := <-confirms; confirmed.Ack {
		log.Println("confirmed delivery with delivery tag: ", confirmed.DeliveryTag)
	} else {
		log.Println("failed delivery of delivery tag: ", confirmed.DeliveryTag)
	}
}
