# Extractor de noticias — Qhubo / Google News RSS

Recolector de noticias por ciudad usando Google News RSS, con desofuscación
de URLs Protobuf y extracción de texto mediante trafilatura + BeautifulSoup
como fallback. Parte de un proyecto de análisis de prensa regional colombiana.

## ¿Qué hace?

Busca artículos por ciudad y rango de fechas en Google News, desencripta las
URLs (formato CBM/CBA), extrae el texto del artículo real y guarda los
resultados en JSON. Funciona con Medellín, Cali y Valledupar, cada una con
su propio script y sus medios locales configurados.

## El problema que resolvió

El recolector original fallaba en el 100% de los artículos de Medellín
(134 de 134). Las URLs nuevas de Google News vienen encriptadas con Protobuf
y algunas tienen protección anti-bot en JS. Lo solucioné integrando
`googlenewsdecoder` para desofuscar nativamente, y agregando un extractor de
emergencia con BeautifulSoup para cuando trafilatura devuelve basura o el
texto de bienvenida de Google en vez del artículo.

## Archivos principales

| Script | Ciudad | Salida |
|---|---|---|
| `news_gratis.py` | Medellín (base) | `noticias_med_2026.json` |
| `recolector_cali.py` | Cali | `noticias_cali_2026_ene_abril7.json` |
| `recolector_valledupar.py` | Valledupar | `noticias_valledupar_2026_ene_abril7.json` |

## Requisitos

```bash
pip install trafilatura beautifulsoup4 googlenewsdecoder requests
```

Opcional: si tienes clave de Groq o Together.ai, configúrala como variable
de entorno para activar limpieza de texto con LLM:

```bash
set GROQ_API_KEY=tu_clave_aqui   # Windows
```

## Uso básico

```bash
python news_gratis.py "https://news.google.com/rss/articles/..."
```

Para correr el recolector completo por ciudad, ejecuta directamente el script
correspondiente. El rango de fechas está configurado para enero 1 – abril 7
de 2026.

## Notas

- Los archivos `.json` de resultados están en `.gitignore` (datos internos).
- Las claves API nunca van en el código, siempre por variable de entorno.
