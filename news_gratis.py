import trafilatura
import json
import sys
from datetime import datetime
import os
import requests


# Configuración LLM (opcional)
# Si tienes clave de Groq o Together.ai, el script la detecta automáticamente.
# Si no, funciona solo con Trafilatura.

if "GROQ_API_KEY" in os.environ:
    try:
        from groq import Groq
        client = Groq(api_key=os.environ.get("GROQ_API_KEY"))
        USE_LLM = "groq"
    except ImportError:
        USE_LLM = None
        print("groq no instalado, corriendo sin LLM")

elif "TOGETHER_API_KEY" in os.environ:
    try:
        from together import Together
        client = Together(api_key=os.environ.get("TOGETHER_API_KEY"))
        USE_LLM = "together"
    except ImportError:
        USE_LLM = None
        print("together no instalado, corriendo sin LLM")

else:
    USE_LLM = None
    print("sin LLM configurado, usando solo trafilatura")


def limpiar_con_llm(texto, titulo_raw, url):
    prompt = f"""Extrae la noticia y devuelve solo JSON con estos campos:
{{
  "title": "...",
  "author": "... o null",
  "date": "YYYY-MM-DD o null",
  "excerpt": "máximo 2 frases",
  "category": "... o null",
  "tags": [],
  "main_image": "url o null",
  "content": "texto limpio sin publicidad"
}}

URL: {url}
Título: {titulo_raw[:200]}
Texto: {texto[:25000]}
"""
    if USE_LLM == "groq":
        response = client.chat.completions.create(
            model="llama-3.1-70b-versatile",
            messages=[{"role": "user", "content": prompt}],
            temperature=0,
            max_tokens=2000
        )
        return response.choices[0].message.content

    elif USE_LLM == "together":
        response = client.chat.completions.create(
            model="meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
            messages=[{"role": "user", "content": prompt}],
            temperature=0,
            max_tokens=2000
        )
        return response.choices[0].message.content


def resolver_url_google(url):
    # Las URLs de Google News (CBM/CBA) vienen encriptadas con protobuf.
    # googlenewsdecoder las resuelve nativamente; si falla, se parsea el HTML.
    if "news.google.com" not in url:
        return url

    try:
        from googlenewsdecoder import new_decoderv1
        decoded = new_decoderv1(url)
        if hasattr(decoded, 'get') and decoded.get('decoded_url'):
            return decoded['decoded_url']
    except ImportError:
        print("falta: pip install googlenewsdecoder")
    except Exception as e:
        print(f"googlenewsdecoder falló ({e}), intentando con BS4...")

    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8',
        }

        r = requests.get(url, headers=headers, allow_redirects=True, timeout=8)
        if "google.com" not in r.url:
            return r.url

        from bs4 import BeautifulSoup
        soup = BeautifulSoup(r.text, 'html.parser')

        blacklist = [
            "google.com", "gstatic.com", "googleusercontent", "google-analytics", "analitica",
            "googletagmanager", "doubleclick", "googlesyndication", "googleadservices",
            "analytics", "gtm.js", "favicon", "facebook", "twitter", "instagram", "youtube", "tiktok", "whatsapp"
        ]

        candidatos = []

        for a in soup.find_all('a', href=True):
            href = a['href']
            if href.startswith('http') and not any(ex in href.lower() for ex in blacklist):
                candidatos.append(href)

        for el in soup.find_all(attrs={"data-url": True}):
            d_url = el['data-url']
            if d_url.startswith('http') and not any(ex in d_url.lower() for ex in blacklist):
                candidatos.append(d_url)

        if candidatos:
            candidatos.sort(key=len, reverse=True)
            return candidatos[0]

    except Exception as e:
        print(f"error resolviendo url: {e}")

    return url


def extraer_noticia(url):
    url_real = resolver_url_google(url)
    print(f"url real: {url_real}")
    print(f"descargando...")

    downloaded = None
    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8',
            'Accept-Language': 'es-ES,es;q=0.9,en;q=0.8',
            'Connection': 'keep-alive',
            'Upgrade-Insecure-Requests': '1',
            'Referer': 'https://www.google.com/'
        }
        with requests.Session() as s:
            response = s.get(url_real, headers=headers, timeout=12)
            if response.status_code == 200:
                if response.encoding == 'ISO-8859-1':
                    response.encoding = 'utf-8'
                downloaded = response.text
                print(f"ok ({len(downloaded)} bytes)")
            else:
                print(f"http {response.status_code}")
    except Exception as e:
        print(f"error en descarga: {e}")

    if not downloaded:
        print("intentando con trafilatura.fetch_url...")
        downloaded = trafilatura.fetch_url(url_real)

    metadata = None
    if downloaded:
        metadata = trafilatura.extract_metadata(downloaded)

    texto = None
    if downloaded:
        texto = trafilatura.extract(
            downloaded,
            include_comments=False,
            include_tables=True,
            include_images=False,
            include_links=False,
            favor_precision=True
        )
        if not texto:
            texto = trafilatura.extract(downloaded, favor_recall=True)

    # fallback con BS4 si trafilatura no extrae nada útil
    if downloaded and (not texto or "Comprehensive up-to-date" in texto):
        print("trafilatura falló, usando BS4...")
        try:
            from bs4 import BeautifulSoup
            soup = BeautifulSoup(downloaded, 'html.parser')
            for s in soup(['script', 'style', 'nav', 'header', 'footer', 'aside']): s.decompose()
            paragraphs = [p.get_text().strip() for p in soup.find_all('p') if len(p.get_text().strip()) > 35]
            if paragraphs:
                texto = "\n\n".join(paragraphs)
                print("recuperado con BS4")
        except: pass

    if not texto:
        if metadata and metadata.description:
            texto = metadata.description
        else:
            texto = "No se pudo extraer el contenido. Ver enlace original."

    titulo = metadata.title if metadata else "Sin título"
    autor = metadata.author if metadata else None
    fecha = metadata.date if metadata else None
    imagen = metadata.image if metadata else None
    sitename = metadata.sitename if metadata else None
    resumen = metadata.description if metadata and metadata.description else (texto[:300] + "...")

    if USE_LLM and len(texto) > 100:
        try:
            json_limpio_str = limpiar_con_llm(texto, titulo, url_real)
            if isinstance(json_limpio_str, str):
                import json as json_lib
                json_limpio = json_lib.loads(json_limpio_str)
            else:
                json_limpio = json_limpio_str
            json_limpio['url_real'] = url_real
            print("procesado con LLM")
            return json_limpio
        except Exception as e:
            print(f"LLM falló ({e})")

    resultado = {
        "title": titulo,
        "author": autor,
        "date": fecha,
        "site": sitename,
        "excerpt": resumen,
        "categories": [],
        "main_image": imagen,
        "content": texto,
        "url_real": url_real
    }

    print(f"listo: {titulo[:60]}")

    try:
        with open("ultima_noticia.json", "w", encoding="utf-8") as f:
            json.dump(resultado, f, indent=2, ensure_ascii=False)
    except: pass

    return resultado


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print('uso: python news_gratis.py "https://url-de-la-noticia.com"')
        sys.exit(1)

    url = sys.argv[1]
    extraer_noticia(url)
