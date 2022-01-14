package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type Bot struct {
	Left  int
	Right int
}

func Plus(w http.ResponseWriter, r *http.Request) {
	var b Bot

	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := b.Left + b.Right
	log.Printf("The plus result is %+v", result)
	w.Write([]byte(strconv.Itoa(result)))
}

func Minus(w http.ResponseWriter, r *http.Request) {
	var b Bot

	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := b.Left - b.Right
	log.Println(result)
	log.Printf("The minus result is %+v", result)

	w.Write([]byte(strconv.Itoa(result)))
}

func Times(w http.ResponseWriter, r *http.Request) {
	var b Bot

	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := b.Left * b.Right
	log.Printf("The time result is %+v", result)
	w.Write([]byte(strconv.Itoa(result)))
}

func Divide(w http.ResponseWriter, r *http.Request) {
	var b Bot

	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := b.Left / b.Right
	log.Printf("The divide result is %+v", result)
	w.Write([]byte(strconv.Itoa(result)))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/plus", Plus)
	mux.HandleFunc("/minus", Minus)
	mux.HandleFunc("/times", Times)
	mux.HandleFunc("/divide", Divide)

	http.ListenAndServe(":8008", mux)
}
