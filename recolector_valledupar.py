import json
from datetime import datetime, timezone
from gnews import GNews
import time
import os
from tqdm import tqdm

from news_gratis import extraer_noticia


class NewsStats:
    def __init__(self):
        self.total_processed = 0
        self.successful = 0
        self.failed = 0

    def update_display(self, current_article=None):
        os.system('cls' if os.name == 'nt' else 'clear')
        print(f"\nEstadísticas (Valledupar):")
        print(f"  procesados: {self.total_processed}")
        print(f"  exitosos:   {self.successful}")
        print(f"  fallidos:   {self.failed}")
        if current_article:
            print(f"  procesando: {current_article[:80]}...")


def load_existing_urls(filename):
    if not os.path.exists(filename):
        return set()
    try:
        with open(filename, 'r', encoding='utf-8') as f:
            existing_news = json.load(f)
            return {article['enlace_original'] for article in existing_news}
    except:
        return set()


def save_failed_article(article_data, error_msg):
    filename = 'articulos_fallidos_valledupar.json'
    failed_articles = []
    if os.path.exists(filename):
        with open(filename, 'r', encoding='utf-8') as f:
            failed_articles = json.load(f)
    failed_articles.append({
        'titulo': article_data.get('title', 'Sin título'),
        'url': article_data.get('url', 'Sin URL'),
        'fecha_intento': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
        'error': str(error_msg)
    })
    with open(filename, 'w', encoding='utf-8') as f:
        json.dump(failed_articles, f, ensure_ascii=False, indent=2)


def collect_news(filename):
    google_news = GNews()
    google_news.language, google_news.country, google_news.max_results = 'es', 'Colombia', 100

    fecha_inicio = datetime(2026, 1, 1, tzinfo=timezone.utc)
    fecha_fin = datetime(2026, 4, 7, tzinfo=timezone.utc)

    stats = NewsStats()
    existing_urls = load_existing_urls(filename)
    all_news = []
    if os.path.exists(filename):
        with open(filename, 'r', encoding='utf-8') as f:
            all_news = json.load(f)

    search_terms = [
        '"riña" "Valledupar"', '"pelea" "Valledupar"', '"altercado" "Valledupar"',
        'site:elpilon.com.co "riña"', 'site:radioguatapuri.com "riña"', 'site:rtanoticias.com "riña"',
        '"herido en riña" "Valledupar"', '"muerto en riña" "Valledupar"', '"enfrentamiento" "Valledupar"'
    ]
    keywords = [
        'riña', 'pelea', 'golpe', 'puñal', 'machete', 'herido', 'lesionado',
        'muerto', 'fallecido', 'atacó', 'golpeó', 'hirió', 'calle', 'barrio',
        'vecino', 'joven', 'hombre', 'mujer', 'discusión', 'altercado', 'enfrentamiento'
    ]

    for term in search_terms:
        google_news.period = '1y'
        news = google_news.get_news(term)
        print(f"encontradas {len(news)} noticias para '{term}'")

        for article in tqdm(news, desc=term, unit="art"):
            try:
                stats.total_processed += 1
                article_date = datetime.strptime(
                    article['published date'], '%a, %d %b %Y %H:%M:%S %Z'
                ).replace(tzinfo=timezone.utc)
                formatted_date_prefix = article_date.strftime('%d%m%y')
                stats.update_display(f"{formatted_date_prefix} {article['title']}")

                if article['url'] in existing_urls:
                    continue
                if not (fecha_inicio <= article_date <= fecha_fin):
                    continue

                found_keywords = [
                    k for k in keywords
                    if k in article['title'].lower() or k in (article.get('description') or "").lower()
                ]
                if not found_keywords:
                    continue

                time.sleep(0.5)
                extracted_data_str = extraer_noticia(article['url'])

                if extracted_data_str:
                    extracted_data = json.loads(extracted_data_str) if isinstance(extracted_data_str, str) else extracted_data_str

                    all_news.append({
                        'titulo': extracted_data.get('title', article['title']),
                        'enlace_original': article['url'],
                        'enlace_real': extracted_data.get('url_real', ''),
                        'fecha_google': article['published date'],
                        'fecha_articulo_extraida': extracted_data.get('date'),
                        'autor_extraido': extracted_data.get('author'),
                        'fuente': article['publisher']['title'],
                        'descripcion_google': article.get('description'),
                        'resumen_extraido': extracted_data.get('excerpt'),
                        'contenido_completo': extracted_data.get('content'),
                        'imagen_principal': extracted_data.get('main_image'),
                        'termino_busqueda': term,
                        'palabras_clave_encontradas': found_keywords
                    })
                    existing_urls.add(article['url'])
                    stats.successful += 1
                    with open(filename, 'w', encoding='utf-8') as f:
                        json.dump(all_news, f, ensure_ascii=False, indent=2)
                else:
                    stats.failed += 1
                    save_failed_article(article, "fallo en extracción")

            except Exception as e:
                stats.failed += 1
                save_failed_article(article, str(e))

    return all_news


if __name__ == "__main__":
    # para usar LLM, setea la variable antes de correr:
    # set GROQ_API_KEY=tu_clave   (Windows)
    # export GROQ_API_KEY=tu_clave (Linux/Mac)
    print("iniciando recolección Valledupar...")
    filename = 'noticias_valledupar_2026_ene_abril7.json'
    collect_news(filename)
    print("listo.")
