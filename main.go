package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

type owmErrorMessage struct {
	Code    json.Number `json:"cod"`
	Message string      `json:"message"`
}

type owmSuccessMessage struct {
	Weather []struct {
		Main string `json:"main"`
	}
	Main struct {
		Temperature float32 `json:"temp"`
	} `json:"main"`
}

type httpError struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (err *httpError) Error() string {
	return err.Message
}

type response struct {
	Status          string `json:"status"`
	Condition       string `json:"condition"`
	TemperatureFeel string `json:"temperature_feel"`
}

func main() {
	KEY := os.Getenv("OWM_KEY")
	if KEY == "" {
		panic("OWM_KEY is not set")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		lat, ok := query["lat"]
		if !ok {
			e(w, "missing query parameter: lat")
			return
		}

		lon, ok := query["lon"]
		if !ok {
			e(w, "missing query parameter: lon")
			return
		}

		latitude, err := strconv.ParseFloat(lat[0], 32)
		if err != nil {
			e(w, "invalid value: lat")
			return
		}

		longitude, err := strconv.ParseFloat(lon[0], 32)
		if err != nil {
			e(w, "invalid value: lon")
			return
		}

		data, err := getWeather(float32(longitude), float32(latitude), KEY)
		if err != nil {
			e(w, err.Error())
			return
		}

		respData, err := json.Marshal(response{
			Condition:       data.Weather[0].Main,
			TemperatureFeel: getTemperature(data.Main.Temperature),
		})
		if err != nil {
			e(w, err.Error())
			return
		}

		w.Write(respData)
	})

	http.ListenAndServe(":8081", mux)
}

func getTemperature(x float32) string {
	switch {
	case bw(x, -4, 0):
		return "freezing"
	case bw(x, 0, 4):
		return "very cold"
	case bw(x, 4, 8):
		return "cold"
	case bw(x, 8, 12):
		return "not so cold"
	case bw(x, 12, 16):
		return "mild"
	case bw(x, 16, 20):
		return "less mild"
	case bw(x, 20, 24):
		return "getting hot"
	default:
		return "HOT"
	}
}

func bw(x, a, b float32) bool {
	return x >= a && x < b
}

func getWeather(longitude, latitude float32, key string) (owmSuccessMessage, error) {
	URL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?units=metric&lat=%f&lon=%f&appid=%s",
		latitude,
		longitude,
		key)

	resp, err := http.Get(URL)
	if err != nil {
		log.Println(fmt.Errorf("get weather: %w", err))
		return owmSuccessMessage{}, errors.New("internal server error")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(fmt.Errorf("read request body: %w", err))
		return owmSuccessMessage{}, errors.New("internal server error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var oError owmErrorMessage
		err = json.Unmarshal(body, &oError)
		if err != nil {
			log.Println(fmt.Errorf("unmarshall error request body: %w", err))
			return owmSuccessMessage{}, errors.New("internal server error")
		}

		log.Println(fmt.Errorf("non-200 response: %v %s", oError.Code, oError.Message))
		return owmSuccessMessage{}, errors.New(oError.Message)
	}

	var data owmSuccessMessage
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Println(fmt.Errorf("unmarshall request body: %w", err))
		return owmSuccessMessage{}, errors.New("internal server error")
	}

	return data, nil
}

func e(w http.ResponseWriter, msg string) {
	hError, err := json.Marshal(httpError{Status: "error", Message: msg})
	if err != nil {
		panic(err)
	}

	w.Write(hError)
	log.Println(msg)
}
