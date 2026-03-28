# simple-abtest

[中文](README.md) | [English](README-EN.md) | [Español](README-ES.md) | [Français](README-FR.md) | [日本語](README-JA.md) | [Deutsch](README-DE.md)

`simple-abtest` est une plateforme d’expérimentation A/B auto-hebergée. Elle regroupe l’administration des expériences, la décision online et l’évaluation locale via SDK afin qu’une équipe garde le contrôle sur ses règles, son trafic et son intégration.

## Ce que la plateforme couvre

- Gestion des expériences par application : chaque application peut porter plusieurs expériences avec nom, description, état et filtre d’entrée.
- Gestion des autorisations : plusieurs utilisateurs, avec droits lecture seule, lecture-écriture ou administration par application.
- Ciblage contextuel : le filtre d’une expérience décide si une requête entre ou non dans le flux d’expérimentation.
- Validation online : la console permet de soumettre un `key` et un `context` pour voir la configuration et les tags réellement renvoyés par le moteur.
- Forced hit par clé : un `key` précis peut être forcé vers un groupe donné pour QA, validation ciblée ou reproduction.
- Expériences multi-layers : une expérience peut exposer plusieurs `layers`, chacune produisant sa propre sortie de configuration.
- Alignement du trafic entre layers : les `segments` décrivent des plages de trafic partagées entre layers, et non des variantes métier.
- Répartition dans un segment : les `groups` sont les unités finales de décision; ils portent la part, la config active et l’historique de config.
- Deux modes d’intégration : appel online au moteur ou décision locale basée sur des snapshots chargés par SDK.

## Console d’administration
### Liste
![](images/list-en.png)
### Détail
![](images/detail-en.png)
### Validation
![](images/verify-en.png)
### Notes
- La console ne vérifie pas encore l’exclusivité des noms ni des entrées `force_hit`. En cas de conflit, le comportement peut devenir ambigu; mieux vaut éviter les recouvrements.
- Pour limiter la polarisation du trafic dans les expériences longues, lorsqu’on ajuste la part d’un groupe non par défaut, le système fait tourner des buckets avec le groupe par défaut sans modifier le pourcentage observé à l’instant T. Le groupe par défaut n’est donc pas toujours un bon groupe de contrôle.
- La visualisation des résultats n’est pas encore intégrée. Les tags retournés par le moteur sont prévus pour être exploités dans votre stack analytique.

## Composants

```
User -> Admin - UI
          \
Client ->- -> Engine
```

- `admin` : console d’administration et API de gestion.
- `engine` : service de décision et de répartition du trafic.
- `sdk-go`, `sdk-java`, `sdk-cpp` : SDK d’évaluation locale.

## Intégration

### Décision online

L’application métier envoie une requête à `engine` et reçoit la configuration retenue ainsi que les tags d’affectation. La page de validation de la console emprunte exactement le même chemin, avec le même délai de propagation.

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

Exemple de réponse :

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

### SDK local

Le SDK recharge périodiquement des snapshots d’expériences puis exécute la décision dans le processus applicatif lui-même. C’est l’option adaptée aux chemins très sollicités ou aux services qui veulent limiter la latence réseau.

Exemple Go :

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

Autres SDK :

- [sdk-java/README.md](sdk-java/README.md)
- [sdk-cpp/README.md](sdk-cpp/README.md)

## Démarrage rapide

Dépendances recommandées :

- Go `1.26+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

Initialisation de la base :

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

Exemple de configuration :

`admin/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
redis:
  address: "127.0.0.1:6379"
  password: ""
  pool_size: 10
  idle_size: 2
redis_prefix: "sab-"
test: false
```

Notes :

- `db` : chaîne de connexion MySQL partagée par `admin` et `engine`.
- `redis.address` : instance Redis utilisée par la console pour les sessions et le cache d’autorisations.
- `redis_prefix` : il est conseillé de définir un préfixe distinct par environnement.
- `test` : avec `true`, la plateforme active des capacités de débogage supplémentaires; cela n’est pas recommandé en production.

`engine/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

Notes :

- `interval_s` : fréquence de rechargement des snapshots depuis MySQL par `engine`. La valeur par défaut recommandée est `300` secondes.

Artefacts de build :

```bash
./build.sh
```

Le script vérifie la présence des environnements Go et Node.js/npm puis produit :

- `bin/admin`
- `bin/engine`
- `ui/dist`

Pour lancer les services, il est recommandé d’utiliser directement les binaires compilés :

```bash
./bin/admin -config admin/config.yaml -port 8001 -ui-resource ./ui/dist -engine http://127.0.0.1:8080
./bin/engine -config engine/config.yaml -port 8080
```
