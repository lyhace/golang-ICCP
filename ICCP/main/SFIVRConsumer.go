// This example declares a durable Exchange, an ephemeral (auto-delete) Queue,
// binds the Queue to the Exchange with a binding key, and consumes every
// message published to that Exchange with that routing key.
//
package main

import (
	"ICCP/util"
	"bytes"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net"
	"strconv"
	"time"
)

var (
	uriSFIVRConsumer          = flag.String("uriSFIVRConsumer", "amqp://iccprequest:iccprequest@iccpmq:5672/iccprequest", "AMQP URI")
	exchangeNameSFIVRConsumer = flag.String("exchangeSFIVRConsumer", "iccprequest", "Durable AMQP exchange name")
	exchangeTypeSFIVRConsumer = flag.String("exchange-typeSFIVRConsumer", "direct", "Exchange type - direct|fanout|topic|x-custom")
	queueSFIVRConsumer        = flag.String("queueSFIVRConsumer", "SFIVRRequest", "Ephemeral AMQP queue name")
	bindingKeySFIVRConsumer   = flag.String("keySFIVRConsumer", "SFIVRRequest", "AMQP binding key")
	consumerTagSFIVRConsumer  = flag.String("consumer-tagSFIVRConsumer", "SFIVRConsumer", "AMQP consumer tag (should not be blank)")
	lifetimeSFIVRConsumer     = flag.Duration("lifetimeSFIVRConsumer", 0*time.Second, "lifetime of process before shutdown (0s=infinite)")
	//生成要访问的url
	noticeUrl = "http://sfivrnotice:15407/CTDNOTICE/CTD?mac="
	//noticeUrl = "http://172.16.35.147:32318/SFIVRRequest?mac="
	//消费日志
	logFileName = "/home/smp/logs/SFIVRConsumer_" + time.Now().Format("2006-01-02") + ".log"
	//logFileName = "SFIVRPublish_" + time.Now().Format("2006-01-02") + ".log"
	//响应异常日志
	logErrorFileName = "/home/smp/logs/SFIVRResponseError_" + time.Now().Format("2006-01-02") + ".log"
	//logErrorFileName = "SFIVRResponseError_" + time.Now().Format("2006-01-02") + ".log"
	timeSleepBean TimeSleepBean
	timeSleepStr  string
)

func init() {
	flag.Parse()
}

func main() {
	c, err := NewConsumer(*uriSFIVRConsumer, *exchangeNameSFIVRConsumer, *exchangeTypeSFIVRConsumer, *queueSFIVRConsumer, *bindingKeySFIVRConsumer, *consumerTagSFIVRConsumer)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if *lifetimeSFIVRConsumer > 0 {
		log.Printf("running for %s", *lifetimeSFIVRConsumer)
		time.Sleep(*lifetimeSFIVRConsumer)
	} else {
		log.Printf("running forever")
		select {}
	}

	log.Printf("shutting down")

	if err := c.Shutdown(); err != nil {
		log.Fatalf("error during shutdown: %s", err)
	}
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}

func NewConsumer(amqpURI, exchange, exchangeType, queueName, key, ctag string) (*Consumer, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	//默认连接MQ	方式
	//log.Printf("dialing %q", amqpURI)
	//c.conn, err = amqp.Dial(amqpURI)
	//if err != nil {
	//	return nil, fmt.Errorf("Dial: %s", err)
	//}

	//连接MQ，超时时间3秒
	c.conn, err = amqp.DialConfig(amqpURI, amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 3*time.Second)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		fmt.Printf("closing: %s", <-c.conn.NotifyClose(make(chan *amqp.Error)))
	}()

	log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	// 获取消费通道
	c.channel.Qos(1, 0, true) // 确保rabbitmq会一个一个发消息

	log.Printf("got Channel, declaring Exchange (%q)", exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	log.Printf("declared Exchange, declaring Queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	log.Printf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, key)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		key,        // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	log.Printf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		false,      // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go handle(deliveries, c.done)

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer log.Printf("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func handle(deliveries <-chan amqp.Delivery, done chan error) {
	for d := range deliveries {
		// 将消费的报文发往外呼接口
		go util.HttpClient(string(d.Body), noticeUrl+"cqt1234", logErrorFileName)

		// 根据每秒并发的数量来控制睡眠时间调整并发值
		timeSleepBean = TimeSleepCtl(timeSleepBean)

		// 发送报文后延迟执行（控制每秒最大执行数量）
		time.Sleep(timeSleepBean.timeSleep * time.Microsecond)

		//将请求报文写入日志文件
		var requestJsonBuffer bytes.Buffer
		requestJsonBuffer.WriteString(time.Now().Format("2006-01-02 15:04:05"))
		requestJsonBuffer.WriteString(" ")
		requestJsonBuffer.WriteString(string(d.Body))
		go util.AppendToFile(logFileName, requestJsonBuffer.String())

		d.Ack(false)
	}
	log.Printf("handle: deliveries channel closed")
	done <- nil
}

type TimeSleepBean struct {
	timeOldUnix int64
	oldCount    int
	nowCount    int
	timeSleep   time.Duration
}

// 根据每秒并发的数量来控制睡眠时间调整并发值
func TimeSleepCtl(timeSleepBean TimeSleepBean) TimeSleepBean {
	// 判断当前调用时是否为同一秒
	var timeNowUnix int64 = time.Now().Unix()

	if timeNowUnix == timeSleepBean.timeOldUnix {
		//上一次执行时间与当次执行时间相同时，当前并发数+1
		timeSleepBean.nowCount++
	} else {
		log.Println("timeSleepBean.nowCount: ", timeSleepBean.nowCount)
		log.Println("timeSleepBean.oldCount: ", timeSleepBean.oldCount)

		//基础数值：1秒，1000ms，1000000us
		var baseNum int
		baseNum = 1000000
		//10倍数值
		var nowCountFmt int = 10
		nowCountFmt = (timeSleepBean.nowCount / 10 * 10)

		log.Println("timeSleepBean.nowCount10: ", nowCountFmt)
		switch {
		case nowCountFmt < 11:
			timeSleepStr = "30000"
		case nowCountFmt < 30:
			if timeSleepBean.nowCount+15 < timeSleepBean.oldCount {
				timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt - 10))
			} else {
				// 如上一轮跳阶值与当前档位一致，允许再次增加并发
				if nowCountFmt == timeSleepBean.oldCount {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 20))
				} else {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 10))
				}
			}
		case nowCountFmt < 80:
			if timeSleepBean.nowCount+15 < timeSleepBean.oldCount {
				timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt - 10))
			} else {
				// 如上一轮跳阶值与当前档位一致，允许再次增加并发
				if nowCountFmt == timeSleepBean.oldCount {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 30))
				} else {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 20))
				}
			}
		case nowCountFmt < 120:
			if timeSleepBean.nowCount+15 < timeSleepBean.oldCount {
				timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt - 10))
			} else {
				// 如上一轮跳阶值与当前档位一致，允许再次增加并发
				if nowCountFmt == timeSleepBean.oldCount {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 40))
				} else {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 30))
				}
			}
		case nowCountFmt < 200:
			if timeSleepBean.nowCount+15 < timeSleepBean.oldCount {
				timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt - 10))
			} else {
				// 如上一轮跳阶值与当前档位一致，允许再次增加并发
				if nowCountFmt == timeSleepBean.oldCount {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 50))
				} else {
					timeSleepStr = strconv.Itoa(baseNum / (nowCountFmt + 40))
				}
			}
		default:
			timeSleepStr = "5000"
		}
		//格式化计算后的延迟微秒数
		timeSleepBean.timeSleep, _ = time.ParseDuration(timeSleepStr + "ns")
		log.Println("timeSleepBean.timeSleep: ", timeSleepBean.timeSleep)

		timeSleepBean.oldCount = nowCountFmt
		timeSleepBean.nowCount = 0
	}

	timeSleepBean.timeOldUnix = timeNowUnix
	return timeSleepBean
}
