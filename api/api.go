package api

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type State bool
type ApiCall func() (State, error)

const On State = true
const Off State = false
const unkonwn State = Off

var StateNames = map[State]string{
	On:  "on",
	Off: "off",
}

type Api struct {
	url            url.URL
	key            string
	LastKnownState State
}

func NewAPI(key string, host string, relay int) *Api {
	return &Api{
		key: key,
		url: url.URL{
			Scheme: "http",
			Host:   host,
			Path:   fmt.Sprintf("/api/relay/%d", relay),
		},
	}
}

func request(method, url string, body io.Reader) (State, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return unkonwn, errors.Wrap(err, "api.request.NewRequest")
	}

	r.Header.Add("Accept", "application/json")
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return unkonwn, errors.Wrap(err, "api.request.Do")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return unkonwn, errors.Wrap(fmt.Errorf("got response status %d", resp.StatusCode), "api.request")
	}

	var m map[string]int
	err = json.NewDecoder(resp.Body).Decode(&m)
	if err != nil {
		return unkonwn, errors.Wrap(err, "api.request.Decode")
	}

	for _, value := range m {
		// return the first (and only) value, ignoring the key
		return value == 1, nil
	}

	return unkonwn, errors.Wrap(fmt.Errorf("no values in response"), "api.request")

}

func (a *Api) Status() (s State, err error) {
	defer func() {
		if err == nil {
			a.LastKnownState = s
		}
	}()
	q := a.url.Query()
	q.Set("apikey", a.key)
	a.url.RawQuery = q.Encode()
	return request(http.MethodGet, a.url.String(), nil)
}

func st2str(s State) string {
	si := int(0)
	if s {
		si = 1
	}
	return strconv.Itoa(si)
}

func (a *Api) Turn(state State) (s State, err error) {
	defer func() {
		if err == nil {
			a.LastKnownState = s
		}
	}()
	data := url.Values{}
	data.Set("apikey", a.key)
	data.Set("value", st2str(state))
	return request(http.MethodPut, a.url.String(), strings.NewReader(data.Encode()))
}

func (a *Api) TurnOn() (s State, err error) {
	return a.Turn(On)
}

func (a *Api) TurnOff() (s State, err error) {
	return a.Turn(Off)
}
