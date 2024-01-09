package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type ResponseObject struct {
	ID       string
	Joke     string
	HttpCode int
}

func main() {
	request, error := http.NewRequest("GET", "https://icanhazdadjoke.com/", nil)
	if error != nil {
		fmt.Print(error.Error())
		os.Exit(1)
	}
	request.Header.Set("User-agent", "https://github.com/lhaig/godad")
	request.Header.Set("Accept", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
	var responseObject ResponseObject
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
	if err := json.Unmarshal(responseData, &responseObject); err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
	}
	fmt.Println(responseObject.Joke)
}
