package restful

import (
	"fmt"
	"github.com/ant0ine/go-json-rest/rest"
)

func writejson(w rest.ResponseWriter, result interface{}) {
	err := w.WriteJson(result)
	if err != nil {
		fmt.Println(fmt.Sprintf("writejson err %s", err))
	}
}
