package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var municipioADep map[string]string

func init() {
	municipoADep = cargarMunicipios()
}

func cortar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func esFechaISO(s string) bool {
	ok, _ := regexp.MatchString(`\d{4}-\d{2}-\d{2}`, s)
	return ok
}

type NoticiaEntrada struct {
	Title          string `json:"titulo"`
	Content        string `json:"contenidocompleto"`
	URL            string `json:"enlacefinal"`
	SourceName     string `json:"fuente"`
	PublishedAt    string `json:"fechagoogle"`
	FechaDelEvento string `json:"fechaarticulo"`
}

type Coordenadas struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type NoticiaSalida struct {
	Title                   string      `json:"title"`
	FechaEvento             string      `json:"fechaevento"`
	Ciudad                  string      `json:"ciudad"`
	Departamento            string      `json:"departamento"`
	Barrio                  string      `json:"barrio"`
	Coordenadas             Coordenadas `json:"coordenadas"`
	TipoEvento              string      `json:"tipoevento"`
	Motivo                  string      `json:"motivo"`
	TipoArma                string      `json:"tipoarma"`
	Heridos                 int         `json:"heridos"`
	Fallecidos              int         `json:"fallecidos"`
	Atendido                bool        `json:"atendido"`
	Hospital                string      `json:"hospital"`
	AutoridadesIntervencion bool        `json:"autoridadesintervencion"`
	TipoAutoridad           string      `json:"tipoautoridad"`
	Fuente                  string      `json:"fuente"`
	UrlFuente               string      `json:"urlfuente"`
}

type EntidadNER struct {
	Text  string `json:"text"`
	Label string `json:"label"`
}

func extraerNER(texto string) ([]EntidadNER, error) {
	body, _ := json.Marshal(map[string]string{"text": texto})
	resp, err := http.Post("http://localhost:5000/ner", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []EntidadNER
	json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}

func cargarNoticias(ruta string) ([]NoticiaEntrada, error) {
	data, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	var n []NoticiaEntrada
	err = json.Unmarshal(data, &n)
	return n, err
}

func guardarNoticias(noticias []NoticiaSalida, ruta string) error {
	data, err := json.MarshalIndent(noticias, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0644)
}

var ciudadesInvalidas = map[string]bool{
	"policia": true, "hospital": true, "barrio": true, "vereda": true,
	"sector": true, "zona": true, "norte": true, "sur": true,
	"colombia": true, "pais": true, "gobierno": true, "alcaldia": true,
	"calle": true, "carrera": true, "avenida": true, "parque": true,
	"mayo": true, "primero": true, "frente": true, "paz": true,
	"disparo": true, "menor": true, "santander": true,
}

var ciudadesAmbiguas = map[string]bool{
	"el paso": true, "la paz": true, "el retiro": true, "la union": true,
	"la vega": true, "la victoria": true, "el banco": true, "el carmen": true,
	"la plata": true, "el dorado": true,
}

func esCiudadValida(ciudad string) bool {
	low := strings.ToLower(ciudad)
	if ciudadesInvalidas[low] {
		return false
	}
	if low == "cartagena" {
		return true
	}
	if ciudadesAmbiguas[low] {
		if ciudad == strings.ToLower(ciudad) {
			return false
		}
	}
	if municipioADep != nil {
		if _, ok := municipioADep[low]; ok {
			return true
		}
		for mun := range municipioADep {
			if strings.HasSuffix(mun, "de "+low) || mun == low {
				return true
			}
		}
	}
	return false
}

func esInternacional(n NoticiaEntrada) bool {
	sample := cortar(n.Content, 600)
	texto := strings.ToLower(n.Title + sample)
	kws := []string{
		"venezuela", "caracas", "cota 905", "quito", "ecuador",
		"mexico", "cdmx", "chile", "santiago de chile", "argentina",
		"buenos aires", "miami", "estados unidos", "espana", "polonia",
		"policia nacional bolivariana", "cpnb", "cicpc",
	}
	for _, kw := range kws {
		if kw == "cota 905" || kw == "caracas" || kw == "venezuela" {
			if strings.Contains(texto, kw) {
				return true
			}
			continue
		}
		if kw == "buenos aires" {
			if strings.Contains(texto, kw) {
				if strings.Contains(texto, "barrio buenos aires") ||
					strings.Contains(texto, "medellin") ||
					strings.Contains(texto, "armenia") {
					continue
				}
				return true
			}
			continue
		}
		if kw == "santiago" {
			if strings.Contains(texto, kw) {
				if strings.Contains(texto, "santiago de tolu") ||
					strings.Contains(texto, "santiago de cali") ||
					strings.Contains(texto, "santiago de armas") {
					continue
				}
				return true
			}
			continue
		}
		if len(kw) > 4 {
			if strings.Contains(texto, kw) {
				return true
			}
		} else {
			re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
			if re.MatchString(texto) {
				return true
			}
		}
	}
	return false
}

func esDeportivo(n NoticiaEntrada) bool {
	sample := cortar(n.Content, 600)
	titulo := strings.ToLower(n.Title)
	texto := titulo + strings.ToLower(sample)
	if strings.Contains(texto, "hincha") ||
		strings.Contains(texto, "barra brava") ||
		strings.Contains(texto, "disturbios") {
		return false
	}
	claves := []string{
		"boxeo", "titulo mundial", "round", "nocaut",
		"fichaje del jugador", "liga betplay", "torneo betplay",
		"partido amistoso", "seleccion colombia",
		"copa america", "eliminatorias sudamericanas",
	}
	for _, k := range claves {
		if strings.Contains(texto, k) {
			return true
		}
	}
	if strings.Contains(texto, "pelea por el titulo") ||
		strings.Contains(texto, "pelea por el puesto") {
		return true
	}
	return false
}

func limpiarContenido(content string) string {
	lines := strings.Split(content, "\n")
	var res []string
	skip := 0
	end := []string{
		"compartir nota", "seguir leyendo", "mas noticias",
		"temas relacionados", "suscribete", "publicidad",
		"sistema integrado de emergencias", "links promovidos",
	}
	skipLine := []string{
		"le puede interesar", "te puede interesar",
		"lea tambien", "ver tambien",
	}
	for _, line := range lines {
		if skip > 0 {
			skip--
			continue
		}
		low := strings.ToLower(strings.TrimSpace(line))
		ok := true
		for _, e := range end {
			if strings.Contains(low, e) {
				return strings.Join(res, "\n")
			}
		}
		for _, s := range skipLine {
			if strings.Contains(low, s) {
				ok = false
				skip = 1
				break
			}
		}
		if ok && len(low) > 3 {
			res = append(res, line)
		}
	}
	return strings.Join(res, "\n")
}

func buscarCiudad(title, content string) string {
	if municipioADep == nil {
		return ""
	}
	titleLow := strings.ToLower(title)
	contentLow := strings.ToLower(content)
	scores := make(map[string]int)
	for mun := range municipioADep {
		if !esCiudadValida(mun) {
			continue
		}
		score := 0
		munLow := strings.ToLower(mun)
		if strings.Contains(titleLow, munLow) {
			score += 10
			if strings.Contains(titleLow, "en "+munLow) {
				score += 15
			}
		}
		if strings.Contains(contentLow, munLow) {
			score += 5
			if idx := strings.Index(contentLow, munLow); idx < 600 {
				score += 25
			}
		}
		reBarrio := regexp.MustCompile(`(?i)\bbarrio\s+` + regexp.QuoteMeta(munLow))
		if reBarrio.MatchString(titleLow) || reBarrio.MatchString(contentLow[:min(500, len(contentLow))]) {
			score -= 50
		}
		if score > 0 {
			scores[mun] = score
		}
	}
	var best string
	var maxScore int
	for mun, s := range scores {
		if s > maxScore || (s == maxScore && len(mun) > len(best)) {
			maxScore = s
			best = mun
		}
	}
	if best != "" {
		return strings.Title(best)
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buscarMotivo(content string) string {
	low := strings.ToLower(content)
	switch {
	case strings.Contains(low, "intolerancia") || strings.Contains(low, "problemas personales"):
		return "intolerancia"
	case strings.Contains(low, "barra brava") || strings.Contains(low, "hincha") && strings.Contains(low, "futbol"):
		return "violencia entre hinchas"
	case strings.Contains(low, "habitante de calle"):
		return "habitantes de calle"
	case strings.Contains(low, "pasional") || strings.Contains(low, "feminicidio") || strings.Contains(low, "ex pareja"):
		return "pasional"
	case strings.Contains(low, "ria carcelaria") || strings.Contains(low, "privados de la libertad"):
		return "ria carcelaria"
	case strings.Contains(low, "alcohol") || strings.Contains(low, "licor") || strings.Contains(low, "embriaguez"):
		return "alcohol"
	case strings.Contains(low, "escolar") || strings.Contains(low, "estudiantes"):
		return "escolar"
	}
	return "pelea callejera"
}

func buscarArma(content string) string {
	low := strings.ToLower(content)
	switch {
	case strings.Contains(low, "disparo") || strings.Contains(low, "baleado") || strings.Contains(low, "bala"):
		return "arma de fuego"
	case strings.Contains(low, "apunalado") || strings.Contains(low, "cuchillo") || strings.Contains(low, "navaja"):
		return "arma blanca"
	case strings.Contains(low, "machete"):
		return "machete"
	}
	return ""
}

func buscarHeridos(content string) int {
	low := strings.ToLower(content)
	nums := map[string]int{
		"un ": 1, "una ": 1, "dos ": 2, "tres ": 3, "cuatro ": 4,
		"cinco ": 5, "seis ": 6, "siete ": 7, "ocho ": 8, "nueve ": 9,
	}
	for txt, n := range nums {
		if strings.Contains(low, txt+"herido") || strings.Contains(low, txt+"lesionado") {
			return n
		}
	}
	re := regexp.MustCompile(`(\d{1,3})\s*(?:personas?)?\s*(?:herid|lesionad)`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			return v
		}
	}
	if strings.Contains(low, "herido") || strings.Contains(low, "lesionado") {
		return 1
	}
	return 0
}

func buscarFallecidos(content string) int {
	low := strings.ToLower(content)
	palabras := []string{"murio", "fallecio", "muerte", "asesinado", "homicidio", "occiso", "muerto"}
	nums := map[string]int{
		"un ": 1, "una ": 1, "dos ": 2, "tres ": 3, "cuatro ": 4,
	}
	for txt, n := range nums {
		for _, p := range palabras {
			if strings.Contains(low, txt+p) {
				return n
			}
		}
	}
	re := regexp.MustCompile(`(\d{1,3})\s*(?:personas?)?\s*(?:muert|fallecid|asesinado)`)
	matches := re.FindAllStringSubmatchIndex(low, -1)
	max := 0
	for _, m := range matches {
		if len(m) >= 4 {
			if v, err := strconv.Atoi(low[m[2]:m[3]]); err == nil && v > max {
				max = v
			}
		}
	}
	if max > 0 {
		return max
	}
	for _, p := range palabras {
		if strings.Contains(low, p) {
			return 1
		}
	}
	return 0
}

func buscarAtencion(content string) (bool, string) {
	re := regexp.MustCompile(`(?i)hospital\s+([A-Za-z]{2,50})`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		nombre := strings.TrimSpace(m[1])
		low := strings.ToLower(nombre)
		for _, inv := range []string{"en ", "de ", "ria"} {
			if strings.HasPrefix(low, inv) {
				nombre = ""
				break
			}
		}
		if len(nombre) > 2 {
			return true, nombre
		}
		return true, ""
	}
	low := strings.ToLower(content)
	for _, kw := range []string{"remitido", "atendido", "traslado", "atencion medica"} {
		if strings.Contains(low, kw) {
			return true, ""
		}
	}
	return false, ""
}

func parsearFecha(fechaStr string) (time.Time, error) {
	formatos := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formatos {
		if t, err := time.Parse(f, fechaStr); err == nil {
			return t, nil
		}
	}
	if val, err := strconv.ParseInt(fechaStr, 10, 64); err == nil {
		return time.Unix(val, 0), nil
	}
	return time.Time{}, fmt.Errorf("formato no reconocido: %s", fechaStr)
}

func obtenerFecha(n NoticiaEntrada, content string) string {
	reMes := regexp.MustCompile(`(?i)(\d{1,2})\s+de\s+(enero|febrero|marzo|abril|mayo|junio|julio|agosto|septiembre|octubre|noviembre|diciembre)(?:\s+(?:de(?:l)?)?\s*(\d{4}))?`)
	meses := map[string]string{
		"enero": "01", "febrero": "02", "marzo": "03", "abril": "04",
		"mayo": "05", "junio": "06", "julio": "07", "agosto": "08",
		"septiembre": "09", "octubre": "10", "noviembre": "11", "diciembre": "12",
	}
	if m := reMes.FindStringSubmatch(content); len(m) >= 3 {
		dia := m[1]
		mesNum := meses[strings.ToLower(m[2])]
		anio := ""
		if len(m) > 3 && m[3] != "" {
			anio = m[3]
			if v, _ := strconv.Atoi(anio); v < 2023 {
				anio = ""
			}
		}
		if anio == "" {
			if t, err := parsearFecha(n.PublishedAt); err == nil {
				anio = fmt.Sprintf("%d", t.Year())
			} else {
				anio = "2024"
			}
		}
		if mesNum != "" {
			return fmt.Sprintf("%02s/%s/%s", dia, mesNum, anio)
		}
	}
	t, err := parsearFecha(n.PublishedAt)
	if err != nil {
		return ""
	}
	texto := strings.ToLower(n.Title + content)
	if strings.Contains(texto, "ayer") || strings.Contains(texto, "anoche") {
		return t.AddDate(0, 0, -1).Format("02/01/2006")
	}
	days := map[string]time.Weekday{
		"domingo": time.Sunday, "lunes": time.Monday, "martes": time.Tuesday,
		"miercoles": time.Wednesday, "jueves": time.Thursday,
		"viernes": time.Friday, "sabado": time.Saturday,
	}
	reDia := regexp.MustCompile(`(?i)pasado\s+(domingo|lunes|martes|miercoles|jueves|viernes|sabado)`)
	if m := reDia.FindStringSubmatch(texto); len(m) > 1 {
		target := days[m[1]]
		cur := t.AddDate(0, 0, -1)
		for i := 0; i < 7; i++ {
			if cur.Weekday() == target {
				return cur.Format("02/01/2006")
			}
			cur = cur.AddDate(0, 0, -1)
		}
	}
	return t.Format("02/01/2006")
}

func buscarDepartamento(ciudad, title, content string) string {
	if ciudad != "" && municipioADep != nil {
		if dept, ok := municipioADep[strings.ToLower(ciudad)]; ok {
			return dept
		}
	}
	low := strings.ToLower(title + content)
	depts := []string{
		"norte de santander", "valle del cauca", "san andres",
		"la guajira", "antioquia", "atlantico", "bolivar", "boyaca",
		"caldas", "caqueta", "cauca", "cesar", "choco", "cordoba",
		"cundinamarca", "huila", "magdalena", "meta", "narino",
		"putumayo", "quindio", "risaralda", "santander", "sucre",
		"tolima", "bogota",
	}
	for _, d := range depts {
		if strings.Contains(low, d) {
			return strings.ToUpper(d)
		}
	}
	return ""
}

func geolocalizarLugar(barrio, ciudad, apiKey string) (Coordenadas, error) {
	query := barrio
	if ciudad != "" {
		query += ", " + ciudad + ", Colombia"
	}
	u := "https://maps.googleapis.com/maps/api/geocode/json?address=" +
		strings.ReplaceAll(query, " ", "+") + "&key=" + apiKey
	resp, err := http.Get(u)
	if err != nil {
		return Coordenadas{}, err
	}
	defer resp.Body.Close()
	var res struct {
		Results []struct {
			Geometry struct {
				Location Coordenadas `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if len(res.Results) == 0 {
		return Coordenadas{}, fmt.Errorf("sin resultados para: %s", query)
	}
	return res.Results[0].Geometry.Location, nil
}

func analizarNoticia(n NoticiaEntrada) NoticiaSalida {
	clean := limpiarContenido(n.Content)
	fecha := obtenerFecha(n, n.Content)
	ciudad := ""
	// Detectar ciudad desde titulo
	reCiudad := regexp.MustCompile(`(?i)\ben\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+){0,2})`)
	if m := reCiudad.FindStringSubmatch(n.Title); len(m) > 1 {
		if esCiudadValida(m[1]) {
			ciudad = m[1]
		}
	}
	if ciudad == "" {
		if strings.Contains(strings.ToLower(n.Title+clean), "armenia") {
			ciudad = "Armenia"
		}
	}
	if ciudad == "" {
		ciudad = buscarCiudad(n.Title, clean)
	}
	// Inferir ciudad por fuente
	if ciudad == "" {
		sl := strings.ToLower(n.SourceName)
		switch {
		case strings.Contains(sl, "cali") && !strings.Contains(sl, "california"):
			ciudad = "Cali"
		case strings.Contains(sl, "medellin") || strings.Contains(sl, "medelln"):
			ciudad = "Medellin"
		case strings.Contains(sl, "barranquilla"):
			ciudad = "Barranquilla"
		case strings.Contains(sl, "eluniversal"):
			ciudad = "Cartagena"
		}
	}
	barrio := ""
	reBarrio := regexp.MustCompile(`(?i)en\s+(?:el\s+)?barrio\s+([a-zA-Z0-9][a-zA-Z0-9\s]{0,40}?)(?:[,\.\n]|en\s|y\s|donde)`)
	if m := reBarrio.FindStringSubmatch(n.Content); len(m) > 1 {
		barrio = strings.TrimSpace(m[1])
	}
	// NER
	entidades, err := extraerNER(clean + " " + n.Title)
	if err == nil {
		for _, e := range entidades {
			switch e.Label {
			case "GPE":
				if ciudad == "" && strings.ToLower(e.Text) != "colombia" {
					ciudad = e.Text
				}
			case "LOC":
				if barrio == "" && !strings.EqualFold(e.Text, ciudad) && len(strings.Fields(e.Text)) <= 5 {
					barrio = e.Text
				}
			}
		}
	}
	// Correcciones finales ciudad
	locBogota := []string{
		"Suba", "Kennedy", "Bosa", "Engativa", "Usaquen", "Chapinero",
		"Fontibon", "Ciudad Bolivar", "San Cristobal", "Usme",
	}
	for _, loc := range locBogota {
		if strings.EqualFold(ciudad, loc) {
			if barrio == "" {
				barrio = ciudad
			}
			ciudad = "Bogota"
			break
		}
	}
	if strings.EqualFold(ciudad, "Campoamor") {
		barrio = "Campoamor"
		ciudad = "Medellin"
	}
	atendido, hospital := buscarAtencion(clean)
	return NoticiaSalida{
		Title:                   n.Title,
		FechaEvento:             fecha,
		Ciudad:                  ciudad,
		Departamento:            buscarDepartamento(ciudad, n.Title, clean),
		Barrio:                  barrio,
		TipoEvento:              "ria",
		Motivo:                  buscarMotivo(clean),
		TipoArma:                buscarArma(clean),
		Heridos:                 buscarHeridos(clean),
		Fallecidos:              buscarFallecidos(clean),
		Atendido:                atendido,
		Hospital:                hospital,
		Fuente:                  n.SourceName,
		UrlFuente:               n.URL,
	}
}

func cargarMunicipios() map[string]string {
	mapeo := make(map[string]string)
	data, err := os.ReadFile("municipiospordepartamento.json")
	if err != nil {
		fmt.Printf("Advertencia: no se pudo cargar municipiospordepartamento.json: %v\n", err)
		return mapeo
	}
	var depts map[string][]struct {
		Municipio string `json:"municipio"`
	}
	if err = json.Unmarshal(data, &depts); err != nil {
		fmt.Printf("Advertencia: error al parsear municipiospordepartamento.json: %v\n", err)
		return mapeo
	}
	for dept, muns := range depts {
		for _, m := range muns {
			mapeo[strings.ToLower(m.Municipio)] = dept
		}
	}
	mapeo["corozal"] = "SUCRE"
	mapeo["cartagena"] = "BOLIVAR"
	mapeo["cucuta"] = "NORTE DE SANTANDER"
	mapeo["cali"] = "VALLE DEL CAUCA"
	mapeo["bogota"] = "BOGOTA, D.C."
	return mapeo
}

func main() {
	apiKey := os.Getenv("GEOCODE_API_KEY")
	input := "primeros501de2 copy.json"
	output := "finalnewspri50.json"
	if len(os.Args) > 1 {
		input = os.Args[1]
	}
	if len(os.Args) > 2 {
		output = os.Args[2]
	}
	entrada, err := cargarNoticias(input)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Cargando %d noticias de %s...\n", len(entrada), input)
	var resultados []NoticiaSalida
	for i, n := range entrada {
		fmt.Printf("[%d/%d] %s\n", i+1, len(entrada), cortar(n.Title, 60))
		if esInternacional(n) || strings.Contains(strings.ToLower(n.URL), ".cl") {
			continue
		}
		if esDeportivo(n) {
			continue
		}
		low := strings.ToLower(n.Title + cortar(n.Content, 800))
		if strings.Contains(low, "abuso sexual") || strings.Contains(low, "explotacion sexual") {
			continue
		}
		salida := analizarNoticia(n)
		if apiKey != "" && (salida.Barrio != "" || salida.Ciudad != "") {
			if coord, err := geolocalizarLugar(salida.Barrio, salida.Ciudad, apiKey); err == nil {
				salida.Coordenadas = coord
			}
		}
		resultados = append(resultados, salida)
	}
	if err := guardarNoticias(resultados, output); err != nil {
		panic(err)
	}
	fmt.Printf("Listo. %d noticias guardadas en %s.\n", len(resultados), output)
}
