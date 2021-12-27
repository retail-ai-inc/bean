{{ .Copyright }}
package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"io"
	"io/ioutil"
	"strings"

	"github.com/labstack/echo/v4"
)

func PostDataStripTags(c echo.Context, trimSpace bool) (map[string]interface{}, error) {

	// Hold the POST json parameters as an interface
	var data interface{}

	// Hold the POST data as a map with string index
	var postdatamap map[string]interface{}

	// Get Content-Type parameter from request header
	contentType := c.Request().Header.Get("Content-Type")

	if strings.ToLower(contentType) == "application/json" {

		// XXX: IMPORTANT - c.Request().Body is a buffer, which means that once it has been read, it cannot be read again.
		if c.Request().Body != nil {

			var err error

			bodyBytes := bytes.NewBuffer(make([]byte, 0))

			reader := io.TeeReader(c.Request().Body, bodyBytes)

			if err = json.NewDecoder(reader).Decode(&data); err != nil {

				var syntaxError *json.SyntaxError
				var unmarshalTypeError *json.UnmarshalTypeError

				// XXX: IMPORTANT - JSON Syntax error handling
				switch {

				// Request body contains badly-formed JSON (at position %d), syntaxError.Offset
				case errors.As(err, &syntaxError):
					return nil, err

				// Request body contains badly-formed JSON
				case errors.Is(err, io.ErrUnexpectedEOF):
					return nil, err

				// Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset
				case errors.As(err, &unmarshalTypeError):
					return nil, err

				// Request body is empty
				case errors.Is(err, io.EOF):
					return nil, err

				default:
					return nil, err
				}
			}

			// Restore the io.ReadCloser to its original state so that we can read c.Request().Body somewhere else
			c.Request().Body = ioutil.NopCloser(bodyBytes)

		} else {

			return nil, errors.New("ERROR: empty request body")
		}

		data = InterfaceStripTags(data, trimSpace)

		// XXX: IMPORTANT - Here we will check again that we able to decode the JSON and load the data into a map[string]interface.
		switch v := data.(type) {

		case map[string]interface{}:
			postdatamap = v
		default:
			return nil, errors.New("ERROR: JSON syntax error.")
		}
	}

	return postdatamap, nil
}

func InterfaceStripTags(data interface{}, trimSpace bool) interface{} {

	if values, ok := data.([]interface{}); ok {

		for i := range values {

			data.([]interface{})[i] = InterfaceStripTags(values[i], trimSpace)
		}

	} else if values, ok := data.(map[string]interface{}); ok {

		for k, v := range values {

			data.(map[string]interface{})[k] = InterfaceStripTags(v, trimSpace)
		}

	} else if value, ok := data.(string); ok {

		if trimSpace {

			value = strings.TrimSpace(value)
		}

		data = html.EscapeString(value)
	}

	return data
}


// Structure is a data type, so you must pass structure address (&) to the following function as the first parameter.
// Example:
//  	test := struct {
// 		Firstname	string
// 		Lastname	string
// 		Age			int
// 	}{
// 		Firstname: "Taro",
// 		Lastname: "<script>alert()</script>Yamada",
// 		Age: 40,
// 	}

// 	helpers.StructStripTags(&test, true)

// 	c.Logger().Info(test.Lastname)
func StructStripTags(data interface{}, trimSpace bool) error {

	bytes, err := json.Marshal(data)

	if err != nil {

		return err
	}

	var mapSI map[string]interface{}

	if err := json.Unmarshal(bytes, &mapSI); err != nil {

		return err
	}

	mapSI = InterfaceStripTags(mapSI, trimSpace).(map[string]interface{})

	bytes2, err := json.Marshal(mapSI)

	if err != nil {

		return err
	}

	if err := json.Unmarshal(bytes2, data); err != nil {

		return err
	}

	return nil
}
