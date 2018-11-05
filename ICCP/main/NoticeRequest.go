package main

import (
	"ICCP/CTDQuest"
	"ICCP/SFIVRRequest"
	"log"
	"net/http"
)

func main() {
	// 顺丰派遣外呼接口
	http.HandleFunc("/SFIVRRequest", SFIVRRequest.RequestMQ)
	log.Println("ListenAndServe: ", "SFIVRRequest")

	// 问卷调查通用接口
	http.HandleFunc("/CTDQUEST", CTDQuest.RequestMQ)
	log.Println("ListenAndServe: ", "CTDQUEST")

	if err := http.ListenAndServe("0.0.0.0:8098", nil); err != nil {
		log.Println("ListenAndServe: ", err)
	}
}
