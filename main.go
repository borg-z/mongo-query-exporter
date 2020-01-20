package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robertkrimen/otto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/alecthomas/kingpin.v2"
)

type Config struct {
	// Конфиг файл
	CollectorName string `yaml:"collector_name"`
	DatabaseURI   string `yaml:"database_uri"`
	Metrics       []struct {
		MetricName string        `yaml:"metric_name"`
		Type       string        `yaml:"type"`
		Function   string        `yaml:"function"`
		Database   string        `yaml:"database"`
		Collection string        `yaml:"collection"`
		Interval   time.Duration `yaml:"interval"`
		Value      string        `yaml:"value"`
		Query      string        `yaml:"query"`
	} `yaml:"metrics"`
}

var (
	port   = kingpin.Flag("port", "Prometheus exporter listen port").Required().Int()
	config = kingpin.Flag("config", "Prometheus exporter listen port").String()
)

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()
	c := NewCollector()
	prometheus.MustRegister(c)

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :", *port)
	log.Fatal(http.ListenAndServe(strings.TrimSpace(fmt.Sprintln(":", *port)), nil))
}

func Normalize(query string) []bson.D {
	// Преобразует js -> json ->bson
	DateRegexp := regexp.MustCompile(`\(?(new\sDate|ISODate)\(.(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}).\)\)?`)
	match := DateRegexp.FindAllStringSubmatch(query, -1)

	// Преобразуем (new Date("2016-01-01 23:59:59")) -> {$date:{$numberLong:"134234234234"}}
	if match != nil {
		log.Println("Trying to convert time: ", match)
		for _, i := range match {
			t, err := time.Parse("2006-01-02 15:04:04", i[2])

			if err != nil {
				log.Fatalln(err, i[2])
			}
			query = strings.Replace(query, i[0], fmt.Sprintf(`{$date:{$numberLong:"%v"}}`, int64(t.Unix()*1000)), 1)

		}
	}

	// Преобразуем js в json
	vm := otto.New()
	jsvalue, err := vm.Run(query)
	output, err := jsvalue.Export()
	// log.Println(fmt.Sprintln(output))
	output2, err := json.Marshal(output)
	// Преобразуем extjson в bson
	bdoc := make([]bson.D, 0)
	err = bson.UnmarshalExtJSON([]byte(output2), true, &bdoc)
	if err != nil {
		log.Fatal("UnmarshalExtyaml: ", err)
	}
	// for _, i := range bdoc {
	// 	log.Println(i.Map())

	// }

	log.Println("BSON query: ", bdoc)
	return bdoc

}

func Connect(ctx context.Context, uri string) *mongo.Client {
	// Подключаемся к базе
	log.Println("Trying connect to database, uri: ", uri)
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected to MongoDB!")
	return client
}

func GetAggregate(ctx context.Context, client *mongo.Client, db string, coll string, query []bson.D, value string) interface{} {
	// Выполняем запрос с функцией aggregate
	collection := client.Database(db).Collection(coll)
	// log.Println(collection.Name())
	res, err := collection.Aggregate(ctx, query)
	if err != nil {
		log.Fatal(err)
	}
	var result []map[string]interface{}
	err = res.All(ctx, &result)
	return result

}
