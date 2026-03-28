# simple-abtest

[中文](README.md) | [English](README-EN.md) | [Español](README-ES.md) | [Français](README-FR.md) | [日本語](README-JA.md) | [Deutsch](README-DE.md)

`simple-abtest` es una plataforma autogestionada de experimentación A/B. Centraliza la administración de experimentos, la decisión online y la evaluación local por SDK para que el equipo controle la configuración, el tráfico y la integración extremo a extremo.

## Qué ofrece

- Gestión de experimentos por aplicación: cada aplicación puede tener varios experimentos con nombre, descripción, estado y reglas de entrada.
- Control de acceso por aplicación: varios usuarios pueden iniciar sesión y recibir permisos de solo lectura, edición o administración.
- Segmentación por contexto: el filtro del experimento decide quién entra según atributos del request.
- Validación online: desde la consola se puede probar un `key` y su `context` para ver qué configuración y etiquetas devuelve el motor.
- Forced hit por clave: un `key` concreto puede fijarse a un grupo para QA, debugging o validación dirigida.
- Experimentos multicapa: un experimento puede tener varias `layers`; cada layer devuelve una configuración independiente.
- Alineación de tráfico entre layers: los `segments` representan rangos de tráfico compartidos entre layers, no variantes de negocio.
- Reparto dentro de cada segmento: los `groups` son la unidad final de decisión y reparten cuota, configuración activa e historial.
- Dos modos de integración: decisión online por API o decisión local con snapshots descargados por SDK.

## Consola de administración
### Lista
![](images/list-en.png)
### Detalle
![](images/detail-en.png)
### Validación
![](images/verify-en.png)
### Notas
- La consola no impone validaciones de exclusividad sobre nombres ni sobre `force_hit`. Si hay conflictos, el comportamiento puede ser ambiguo; conviene evitar solapamientos.
- En experimentos largos, al redistribuir cuota de un grupo no predeterminado, el sistema rota buckets con el grupo por defecto sin cambiar el porcentaje efectivo del momento. Por eso, el grupo por defecto no siempre es el mejor grupo de control.
- La plataforma todavía no incluye visualización de resultados. Las etiquetas devueltas por el motor están pensadas para enlazar con tu stack analítico.

## Componentes

```
User -> Admin - UI
          \
Client ->- -> Engine
```

- `admin`: consola de administración y API de backoffice.
- `engine`: servicio de decisión y distribución de tráfico.
- `sdk-go`, `sdk-java`, `sdk-cpp`: SDK para evaluación local.

## Cómo integrarlo

### Decisión online

La aplicación de negocio envía un request a `engine` y recibe la configuración ganadora y las etiquetas de asignación. La página de validación de la consola usa la misma ruta, así que comparte el mismo retardo de propagación.

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

Respuesta de ejemplo:

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

El SDK descarga snapshots de experimentos de forma periódica y resuelve la asignación dentro del propio proceso de negocio. Es la opción adecuada cuando la ruta de decisión se ejecuta con mucha frecuencia o se quiere reducir latencia y dependencias de red.

Ejemplo en Go:

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

Más documentación de SDK:

- [sdk-java/README.md](sdk-java/README.md)
- [sdk-cpp/README.md](sdk-cpp/README.md)

## Puesta en marcha

Dependencias recomendadas:

- Go `1.26+`
- Node.js `22+`
- MySQL `8+`
- Redis `6+`

Inicializa la base de datos:

```bash
mysql -uroot -p abtest < db/admin.sql
mysql -uroot -p abtest < db/engine.sql
```

Ejemplo de configuración:

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

Notas:

- `db`: cadena de conexión MySQL compartida por `admin` y `engine`.
- `redis.address`: instancia Redis usada por la consola para sesiones y caché de permisos.
- `redis_prefix`: conviene definir un prefijo distinto por entorno para evitar colisiones.
- `test`: con `true` se activan capacidades adicionales de depuración; no se recomienda en producción.

`engine/config.yaml`

```yaml
db: "abtest:abtest@tcp(127.0.0.1:3306)/abtest?parseTime=true&charset=utf8mb4"
interval_s: 300
```

Notas:

- `interval_s`: frecuencia con la que `engine` vuelve a cargar snapshots desde MySQL. El valor recomendado por defecto es `300` segundos.

Artefactos de build:

```bash
./build.sh
```

El script comprueba el entorno local de Go y Node.js/npm y genera:

- `bin/admin`
- `bin/engine`
- `ui/dist`

Para levantar los servicios se recomienda usar directamente los binarios compilados:

```bash
./bin/admin -config admin/config.yaml -port 8001 -ui-resource ./ui/dist -engine http://127.0.0.1:8080
./bin/engine -config engine/config.yaml -port 8080
```
