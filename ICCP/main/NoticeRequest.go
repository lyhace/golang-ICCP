package main

import (
	"ICCP/CTDQuest"
	"ICCP/SFIVRRequest"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/SFIVRRequest", SFIVRRequest.RequestMQ)
	log.Println("ListenAndServe: ", "SFIVRRequest")

	http.HandleFunc("/CTDQUEST", CTDQuest.RequestMQ)
	log.Println("ListenAndServe: ", "CTDQUEST")

	if err := http.ListenAndServe("0.0.0.0:8098", nil); err != nil {
		log.Println("ListenAndServe: ", err)
	}
}
