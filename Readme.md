# mongo-query-exporter

Prometheus экспортёр, который выполняет запросы в mongodb и возвращает метрики.


## Сборка 

Для зависимостей используется [go-dep](https://github.com/golang/dep)

        dep ensure
        go build -o mongo-query-exporter  main.go collector.go

## Запуск 

                ./mongo-query-exporter --help 
                usage: mongo-query-exporter --port=PORT [<flags>]

                Flags:
                --help           Show context-sensitive help (also try --help-long and --help-man).
                --port=PORT      Prometheus exporter listen port
                --config=CONFIG  Prometheus exporter listen port
                --version        Show application version.


`--port` обязательный аргумент

`--config` по умолчанию используется config.yaml, можно указать путь к другому файлу.


## Конфигурация

### Пример
```Yaml

collector_name: mongo-dev
database_uri: mongodb://mongo.dev/
metrics:
  - metric_name: esia_users # Название метрики
    type: gauge # Не импользуется
    function: aggregate # Не импользуется
    database: user # База данных
    collection: user # Коллекция
    interval: 5s # Время жизни кэша. После выполнения запроса он попадает в кэш на время interval. 
    value: count # Значение. `{ "_id" : null, "count" : 20 }`
    query: |
      [ {$match: {$and:[{"some_user_id": {$ne: null}}, { $or: [{"some_date": {$lte: "2019-02-28 23:59:59"} },{"some_date": null}]}]}}, { $group: { _id: "$id", count: {$sum:1} } }]
```

interval - в формате golang time.Duration. Например можно использовать:  `1s,1m,1h,1d`

### Дата/Время

Поддерживается указание даты в форматах:

```js 
(new Date("2016-01-01 23:59:59"))
ISODate("2019-01-01 23:59:59")
```
Указанные даты парсятся, конвертируюся в EXT JSON в формате `{$date:{$numberLong:"134234234234"}}`, далее преобразуются в bson запрос.