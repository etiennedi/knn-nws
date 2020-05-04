package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
)

const esIndexName = "vectors"

func setMappings() error {
	body := []byte(`
{
    "settings" : {
        "number_of_shards" : 1
    },
    "mappings" : {
        "properties" : {
				    "name": { "type": "text" },
						"embedding_vector" : { "type" : "binary", "doc_values": true }
        }
    }
}`)

	br := bytes.NewBuffer(body)
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:9201/%s", esIndexName), br)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("content-type", "application/json")
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("mappings: status is %s", res.Status)
	}

	return nil
}

func storeToES(id int, name string, vector []float32) error {
	body := []byte(fmt.Sprintf(`
{
  "name": "%s",
  "embedding_vector": "%s"
}
`, name, string(convertArrayToBase64(vector))))

	br := bytes.NewBuffer(body)
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:9201/%s/_doc/%d", esIndexName, id), br)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Add("content-type", "application/json")
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != 201 && res.StatusCode != 200 {
		resbody, _ := ioutil.ReadAll(res.Body)
		fmt.Print(string(resbody))
		return fmt.Errorf("insert: status is %s", res.Status)
	}

	return nil
}

func convertArrayToBase64(array []float32) string {
	bytes := make([]byte, 0, 4*len(array))
	for _, a := range array {
		bits := math.Float32bits(a)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, bits)
		bytes = append(bytes, b...)
	}

	encoded := base64.StdEncoding.EncodeToString(bytes)
	return encoded
}

func convertBase64ToArray(base64Str string) ([]float32, error) {
	decoded, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, err
	}

	length := len(decoded)
	array := make([]float32, 0, length/4)

	for i := 0; i < len(decoded); i += 4 {
		bits := binary.BigEndian.Uint32(decoded[i : i+4])
		f := math.Float32frombits(bits)
		array = append(array, f)
	}
	return array, nil
}
