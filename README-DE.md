# simple-abtest

[中文](README.md) | [English](README-EN.md) | [Español](README-ES.md) | [Français](README-FR.md) | [日本語](README-JA.md) | [Deutsch](README-DE.md)

`simple-abtest` ist eine selbst betriebene A/B-Experimentplattform. Sie bündelt Experimentverwaltung, Online-Entscheidung und lokale Auswertung per SDK, damit ein Team Regeln, Traffic und Integrationspfade durchgängig selbst steuern kann.

## Funktionsumfang

- Experimentverwaltung pro Anwendung: Eine Anwendung kann mehrere Experimente mit Name, Beschreibung, Status und Eintrittsfilter besitzen.
- Rechteverwaltung pro Anwendung: Mehrere Benutzer mit Rollen fur Lesen, Bearbeiten oder Administration.
- Kontextbasiertes Targeting: Definieren Sie Filterbedingungen anhand des fachlichen Kontexts, damit nur passende Requests in ein Experiment gelangen.
- Validierung: In der Konsole lassen sich `key` und `context` eingeben, um das tatsachliche Ergebnis des Dienstes zu prufen.
- Forced hit pro Schlussel: Ein bestimmter `key` kann fest auf eine Gruppe gelenkt werden, etwa fur QA, Abnahme oder Reproduktion.
- Mehrschichtige Experimente: Ein Experiment kann mehrere `layers` enthalten; jede Layer liefert ihre eigene Konfigurationsausgabe.
- Traffic-Ausrichtung uber Layers hinweg: `segments` stehen fur gemeinsam genutzte Traffic-Bereiche zwischen Layers, nicht fur fachliche Varianten.
- Verteilung innerhalb eines Segments: `groups` sind die eigentlichen Entscheidungseinheiten und tragen Anteil, aktive Konfiguration und Konfigurationshistorie.
- Zwei Integrationswege: Online-Entscheidung uber `engine` oder lokale Auswertung uber SDK-Snapshots.

## Admin-Konsole
### Liste
![](images/list-en.png)
### Detail
![](images/detail-en.png)
### Validierungsseite
![](images/verify-en.png)
### Hinweise
- Die Konsole erzwingt derzeit keine Exklusivitatsprufung fur Namen oder `force_hit`. Bei Kollisionen kann das Verhalten mehrdeutig werden; uberschneidende Einstellungen sollten vermieden werden.
- Um Traffic-Polarisierung in Langzeitexperimenten zu reduzieren, werden beim Anpassen eines nicht standardmassigen Gruppenanteils Buckets mit der Default-Gruppe rotiert, ohne das aktuelle Verhaltnis sofort zu verandern. Deshalb ist die Default-Gruppe nicht immer die beste Kontrollgruppe.
- Die Validierung ruft den Entscheidungsdienst direkt auf. Anderungen aus der Konsole werden dort daher ebenfalls erst mit Verzogerung sichtbar.
- Eine eingebaute Ergebnisvisualisierung gibt es noch nicht. Die vom Motor gelieferten Tags sind fur die Weiterverarbeitung in vorhandenen Analytics- oder Observability-Systemen gedacht.

## Komponenten

```
User -> Admin - UI
          \
Client ->- -> Engine
```

- `admin`: Admin-Konsole und Betriebs-API.
- `engine`: Entscheidungs- und Traffic-Verteilungsdienst.
- `sdk-go`, `sdk-java`, `sdk-cpp`: SDKs fur lokale Auswertung.

## Integration

### Online-Entscheidung

Die Fachanwendung sendet Requests an `engine` und erhalt die gewahlte Konfiguration samt Zuordnungstags. Die Validierungsseite der Konsole nutzt denselben Pfad und hat daher dieselbe Propagationsverzogerung.

```http
POST /
ACCESS_TOKEN: <app-access-token>
Content-Type: application/json
```

```json
{
  "appid": 1001,
  "key": "user-123",
  "context": {
    "country": "CN",
    "platform": "ios"
  }
}
```

Beispielantwort:

```json
{
  "config": {
    "feed_rank": "{\"version\":\"B\"}",
    "card_style": "{\"style\":\"large\"}"
  },
  "tags": [
    "feed_rank:variant_b",
    "card_style:control"
  ]
}
```

### Lokales SDK

Das SDK laedt regelmassig Experimentsnapshots und fuhrt die Entscheidung direkt im Prozess der Anwendung aus. Das eignet sich besonders fur stark frequentierte Entscheidungswege oder wenn Netzabhangigkeiten reduziert werden sollen.

Go-Beispiel:

```go
package main

import (
	"fmt"
	"time"

	sdk "github.com/peterrk/simple-abtest/sdk-go"
)

func main() {
	client, err := sdk.NewClient("http://127.0.0.1:8080", 1001, "your-token", 5*time.Minute)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	cfg, tags := client.AB("user-123", map[string]string{
		"country":  "CN",
		"platform": "ios",
	})
	fmt.Println(cfg)
	fmt.Println(tags)
}
```

Weitere SDK-Dokumente:

- [sdk-java/README.md](sdk-java/README.md)
- [sdk-cpp/README.md](sdk-cpp/README.md)

## Schnellstart

Empfohlene Abhangigkeiten:

- Go `1.26+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

Datenbank initialisieren:

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

Beispielkonfiguration:

`admin/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
redis:
  address: "127.0.0.1:6379"
  password: ""
  pool_size: 10
  idle_size: 2
redis_prefix: "sab-"
secret: ""
engine: "http://127.0.0.1:8080"
test: false
```

Hinweise:

- `db`: gemeinsame MySQL-Verbindungszeichenfolge fur `admin` und `engine`.
- `redis.address`: Redis-Instanz fur Sessions und Berechtigungs-Cache der Konsole.
- `redis_prefix`: Es empfiehlt sich ein eigener Prefix je Umgebung, um Kollisionen zu vermeiden.
- `secret`: optionales vordefiniertes Signatur-Secret. Leer lassen, um das bisherige Standardverhalten beizubehalten.
- `engine`: Basis-URL von `engine`, die `admin` fur Online-Verifikation und ahnliche Proxy-Anfragen verwendet. Leer bedeutet Standardwert `http://127.0.0.1:8080`.
- `test`: Mit `true` werden zusatzliche Debug-Funktionen freigeschaltet; fur Produktion nicht empfohlen.

`engine/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

Hinweise:

- `interval_s`: Intervall, in dem `engine` neue Experimentsnapshots aus MySQL nachlaedt. Der empfohlene Standardwert ist `300` Sekunden.

Build-Artefakte:

```bash
./build.sh
```

Das Skript pruft die lokale Go- und Node.js/npm-Umgebung und erzeugt anschliessend:

- `bin/admin`
- `bin/engine`
- `ui/dist`

Fur den Start der Dienste empfiehlt es sich, die gebauten Binardateien direkt zu verwenden:

```bash
./bin/admin -config admin/config.yaml -port 8001 -ui-resource ./ui/dist
./bin/engine -config engine/config.yaml -port 8080
```
