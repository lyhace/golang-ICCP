package CTDQuest

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
	"strconv"
	"strings"
	"time"
)

var (
	uriCTDQuest          = flag.String("uriCTDQuest", "amqp://iccprequest:iccprequest@iccpmq:5672/iccprequest", "AMQP URI")
	exchangeNameCTDQuest = flag.String("exchangeCTDQuest", "iccprequest", "Durable AMQP exchange name")
	exchangeTypeCTDQuest = flag.String("exchange-typeCTDQuest", "direct", "Exchange type - direct|fanout|topic|x-custom")
	routingKeyCTDQuest   = flag.String("keyCTDQuest", "CTDQuestRequest", "AMQP routing key")
	reliableCTDQuest     = flag.Bool("reliableCTDQuest", true, "Wait for the publisher confirmation before exiting")
	//请求日志
	logFileName = "/home/smp/logs/CTDQuestPublish_" + time.Now().Format("2006-01-02") + ".log"
	//logFileName = "CTDQuestPublish_" + time.Now().Format("2006-01-02") + ".log"
	//异常响应日志
	logFileNameResponse = "/home/smp/logs/CTDQuestResponseError_" + time.Now().Format("2006-01-02") + ".log"
	//logFileNameResponse = "CTDQuestResponseError_" + time.Now().Format("2006-01-02") + ".log"
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
	Body   RequestHeader `json:"header"`
	Header RequestBody   `json:"body"`
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
}

type ResponseJson struct {
	Body   ResponseHeader `json:"header"`
	Header ResponseBody   `json:"body"`
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

		requestJsonStr, requestJsonErr := json.Marshal(requestJson)
		checkErrs(requestJsonErr)
		log.Println("requestJsonStr: ", string(requestJsonStr))

		//将请求报文写入日志文件
		var requestJsonBuffer bytes.Buffer
		requestJsonBuffer.WriteString(time.Now().Format("2006-01-02 15:04:05"))
		requestJsonBuffer.WriteString(" ")
		requestJsonBuffer.WriteString(string(requestJsonStr))
		go util.AppendToFile(logFileName, requestJsonBuffer.String())

		var responseJson ResponseJson
		responseJson.Header.Result = "0000"
		responseJson.Header.Reason = "succ"
		responseJson.Body.MessageId = requestJson.Body.MessageId
		responseJson.Body.ServiceName = "IVRResponse"

		// 获取URL里携带的mac值
		queryForm, err := url.ParseQuery(r.URL.RawQuery)
		if err == nil && len(queryForm["mac"]) > 0 {

			if queryForm["mac"][0] != "cqt1234" {
				// 校验mac值是否正确
				firstHash := sha256.New()
				firstHash.Write([]byte(string(requestJsonStr)))
				firstHashStr := hex.EncodeToString(firstHash.Sum(nil))
				firstHashStr += "3F6258C8ADB64B463C31D1A826CBDBF4"
				fmt.Println("mac=", firstHashStr)
				secondHash := sha256.New()
				secondHash.Write([]byte(firstHashStr))
				secondHashStr := hex.EncodeToString(secondHash.Sum(nil))
				fmt.Println("mac=", secondHashStr)

				if secondHashStr != queryForm["mac"][0] {
					responseJson.Header.Result = "0001"
					responseJson.Header.Reason = "mac is error"
				}
				fmt.Println("getmac=", queryForm["mac"][0])
			}

		} else {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "mac is error"
		}

		//判断serviceName是否正确
		if requestJson.Body.ServiceName != "SFIVRRequest" {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "serviceName is error : " + requestJson.Body.ServiceName
		}

		//判断messageId是否为空
		if !(len(requestJson.Body.MessageId) > 0) {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "messageId Is Null!"
		}

		//判断DisplayNum是否为空
		if !(len(requestJson.Header.DisplayNum) > 0) {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "displayNumber Is Null!"
		} else {
			//判断DisplayNum是否为数字
			_, error := strconv.Atoi(requestJson.Header.DisplayNum)
			if error != nil {
				responseJson.Header.Result = "0001"
				responseJson.Header.Reason = "displayNum is error : " + requestJson.Header.DisplayNum
			}
		}

		//判断mediaContent是否为空
		if !(len(requestJson.Header.MediaContent) > 0) {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "mediaContent Is Null!"
		}

		//判断numCode是否为空
		if !(len(requestJson.Header.NumCode) > 0) {
			responseJson.Header.Result = "0001"
			responseJson.Header.Reason = "numCode Is Null!"
		} else {
			//判断numCode是否为数字
			_, error := strconv.Atoi(requestJson.Header.NumCode)
			if error != nil {
				responseJson.Header.Result = "0001"
				responseJson.Header.Reason = "numCode must be a number : " + requestJson.Header.NumCode
			}
		}

		if responseJson.Header.Result == "0000" {
			if err := publish(*uriCTDQuest, *exchangeNameCTDQuest, *exchangeTypeCTDQuest, *routingKeyCTDQuest, string(requestJsonStr), *reliableCTDQuest); err != nil {
				responseJson.Header.Result = "9999"
				responseJson.Header.Reason = "system error"
				if strings.Contains(err.Error(), "i/o timeout") {
					responseJson.Header.Reason = "mq connection timeout"
				}
				log.Println("rabbitmq publish: ", err)
			}
		}

		responseJsonStr, responseJsonErr := json.Marshal(responseJson)
		checkErrs(responseJsonErr)

		//将请求异常报文写入日志文件
		if responseJson.Header.Result != "0000" {
			var requestJsonBufferResponse bytes.Buffer
			requestJsonBufferResponse.WriteString(time.Now().Format("2006-01-02 15:04:05"))
			requestJsonBufferResponse.WriteString(" OUT ")
			requestJsonBufferResponse.WriteString(string(responseJsonStr))
			go util.AppendToFile(logFileNameResponse, requestJsonBufferResponse.String())
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
