package util

import (
	"bytes"
	"fmt"
	"log"
	"os"
)

func AppendToFile(fileName string, content string) error {

	// 判断文件是否存在，不存在则创建
	_, err := os.Stat(fileName)
	if err != nil {
		os.Create(fileName)
	}
	// 以只写的模式，打开文件
	f, err := os.OpenFile(fileName, os.O_WRONLY, 0755)
	if err != nil {
		fmt.Println("cacheFileList.yml file create failed. err: " + err.Error())
	} else {
		var buffer bytes.Buffer
		buffer.WriteString(content)
		buffer.WriteString("\n")
		// 查找文件末尾的偏移量
		n, _ := f.Seek(0, os.SEEK_END)
		// 从末尾的偏移量开始写入内容
		_, err = f.WriteAt([]byte(buffer.String()), n)
		if err != nil {
			log.Println("appendToFile error: ", err)
		}
	}
	defer f.Close()
	return err
}
