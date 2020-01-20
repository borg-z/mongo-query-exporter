package main

import (
	"context"
	"io/ioutil"
	"log"
	"reflect"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gopkg.in/yaml.v2"
)

//Define a struct for you collector that contains pointers
//to prometheus descriptors for each metric you wish to expose.
//Note you can also include fields of other types if they provide utility
//but we just won't be exposing them as metrics.

type Querie struct {
	// Запрос
	name       string
	metric     *prometheus.Desc
	db         string
	collection string
	q          []bson.D
	value      string
	function   string
	interval   time.Duration
}

type Collector struct {
	// Коллектор
	queries []*Querie
	client  *mongo.Client
	ctx     context.Context
	cache   *cache.Cache
}

func NewCollector() *Collector {
	// Инициирем метрики
	var configFile string
	if *config == "" {
		configFile = "config.yaml"
	} else {
		configFile = *config
	}

	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	conf := Config{}
	err = yaml.Unmarshal(buf, &conf)
	if err != nil {
		log.Fatal("Config parsing error")
	}
	ctx := context.Background()
	client := Connect(ctx, conf.DatabaseURI)

	// Инициируем кэш
	c := cache.New(cache.NoExpiration, cache.NoExpiration)

	MongoCollector := &Collector{}
	MongoCollector.client = client
	MongoCollector.ctx = ctx
	MongoCollector.cache = c

	q := &Querie{}
	for _, m := range conf.Metrics {
		q = &Querie{
			name:       m.MetricName,
			metric:     prometheus.NewDesc(m.MetricName, m.Query, []string{"field"}, nil),
			db:         m.Database,
			collection: m.Collection,
			q:          Normalize(m.Query),
			value:      m.Value,
			function:   m.Function,
			interval:   m.Interval,
		}
		MongoCollector.queries = append(MongoCollector.queries, q)

	}

	return MongoCollector
}

//Each and every collector must implement the Describe function.
//It essentially writes all descriptors to the prometheus desc channel.
func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {

	for _, m := range collector.queries {
		ch <- m.metric
	}

}

//Collect implements required collect function for all promehteus collectors
func (collector *Collector) Collect(ch chan<- prometheus.Metric) {
	// Берём метрики из канала, получаем для них данные

	c := collector.cache
	// log.Println("ItemCount: ", c.ItemCount())
	for _, m := range collector.queries {
		var result interface{}
		// var s reflect.Value
		// log.Println(m.name)
		// Проверяем наличие метрики в кэше
		CachedValue, found := c.Get(m.name)
		if found {
			// fmt.Println("Get from cache: ", CachedValue)
			result = CachedValue
			// s = reflect.ValueOf(result)

		} else {
			// Добавляем метрику в кэш на время interval
			// fmt.Println("Cache not found, making querie")
			result = GetAggregate(collector.ctx, collector.client, m.db, m.collection, m.q, m.value)
			c.Set(m.name, result, m.interval)
			// s = reflect.ValueOf(result)
		}

		// log.Println(s.Len())
		mlist := result.([]map[string]interface{})
		if len(mlist) == 1 {
			v := reflect.ValueOf(mlist[0][m.value]).Int()
			ch <- prometheus.MustNewConstMetric(m.metric, prometheus.GaugeValue, float64(v), "empty")

		} else {
			for _, j := range mlist {

				v := reflect.ValueOf(j[m.value]).Int()
				label := reflect.ValueOf(j["_id"]).String()
				ch <- prometheus.MustNewConstMetric(m.metric, prometheus.GaugeValue, float64(v), label)
			}
		}

	}

}
