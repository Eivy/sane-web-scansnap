package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/go-ini/ini"
)

const iniPath string = "/usr/share/scanbd/scripts/scanimage.ini"

func main() {
	var port string
	flag.StringVar(&port, "port", "8080", "port number")
	flag.Parse()
	http.HandleFunc("/save", save)
	http.HandleFunc("/scan", scan)
	http.HandleFunc("/", show)
	http.ListenAndServe(":"+port, nil)
}

func show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var s setting
	_, err := os.Stat(iniPath)
	if err == nil {
		err = ini.MapTo(&s, iniPath)
		if err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	tm, err := template.New("").Parse(html)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = tm.Execute(w, s)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func save(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.ParseForm()
	fmt.Println(r)
	var s setting
	s.Format = r.Form.Get("format")
	s.Mode = r.Form.Get("mode")
	s.Resolution = r.Form.Get("resolution")
	s.Source = r.Form.Get("source")
	s.Size = r.Form.Get("size")
	cfg := ini.Empty()
	err := ini.ReflectFrom(cfg, &s)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ini.PrettyFormat = false
	f, err := os.Create(iniPath)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	cfg.WriteTo(f)
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("location", r.Header.Get("referer"))
	w.WriteHeader(http.StatusFound)
}

func scan(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("scanimage", "-L").Output()
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e := strings.Index(string(out), "'")
	s := strings.Index(string(out), "`")
	if s < 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var se setting
	err = ini.MapTo(&se, iniPath)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var height, width string
	switch se.Size {
	case "A4":
		height = "297"
		width = "210"
	case "A5v":
		height = "210"
		width = "148"
	case "A5h":
		width = "210"
		height = "148"
	case "B5":
		width = "182"
		height = "257"
	}
	device := string(out[0:e][s+1:])
	go func() {
		time.Sleep(time.Second)
		date := time.Now().Format("2006-01-02_150405")
		fmt.Println("scanimage", "-d", device, fmt.Sprintf("--batch=scan-%s-%%d.png", date), fmt.Sprintf("--format=%s", se.Format), fmt.Sprintf("--resolution=%s", se.Resolution), fmt.Sprintf("--mode=%s", se.Mode), fmt.Sprintf("--source=%s", se.Source), fmt.Sprintf("--page-height=%s", height), fmt.Sprintf("--page-width=%s", width))
		out, err = exec.Command("scanimage", "-d", device, fmt.Sprintf("--batch=scan-%s-%%d.png", date), fmt.Sprintf("--format=%s", se.Format), fmt.Sprintf("--resolution=%s", se.Resolution), fmt.Sprintf("--mode=%s", se.Mode), fmt.Sprintf("--source=%s", se.Source), fmt.Sprintf("--page-height=%s", height), fmt.Sprintf("--page-width=%s", width)).Output()
		if err != nil {
			log.Println(err)
			return
		}
	}()
}

const html = `
<!doctype html>
<html lang="ja">
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<title>ScanSnap S1500</title>
	</head>
	<script>
		function start(event) {
			var form = new FormData(document.getElementById('form'));
			var query = new URLSearchParams(form);
			fetch(document.location.href + 'save', {
				method: 'POST',
				body: query,
			})
			.then( function (r) {
				console.log("保存リクエスト完了")
				scan();
			})
			.catch( function(e) {
				alert(e);
			});
		}
		function requested(evt) {
			alert("スキャンを開始しました");
		}
		function scan(event) {
			fetch(document.location.href + "scan")
			.then(function(r) {
				requested();
			})
			.catch(function(e) {
				alert(e);
			});
		}
		function save() {
			var form = new FormData(document.getElementById('form'));
			var query = new URLSearchParams(form);
			fetch(document.location.href + 'save', {
				method: 'POST',
				body: query,
			});
			return false;
		}
	</script>
	<body>
		<h1>ScanSnap S1500 スキャン設定</h1>
		<form id="form" onsubmit="return save();">
			<div>
				<label for="source">読み取り面</label>
				<select id="source" name="source">
					<option {{ if eq .Source "ADF Front" }} selected {{ end }} value='"ADF Front"'>下面</option>
					<option {{ if eq .Source "ADF Back" }} selected {{ end }} value='"ADF Back"'>上面</option>
					<option {{ if eq .Source "ADF Duplex" }} selected {{ end }} value='"ADF Duplex"'>両面</option>
				</select>
			</div>
			<div>
				<label for="mode">カラーモード</label>
				<select id="mode" name="mode">
					<option {{ if eq .Mode "Color" }} selected {{ end }} value="Color">カラー</option>
					<option {{ if eq .Mode "Lineart" }} selected {{ end }} value="Lineart">白黒</option>
					<option {{ if eq .Mode "Halftone" }} selected {{ end }} value="Halftone">ハーフトーン</option>
					<option {{ if eq .Mode "Gray" }} selected {{ end }} value="Gray">グレースケール</option>
				</select>
			</div>
			<div>
			<label for="format">フォーマット</label>
				<select id="format" name="format">
					<option {{ if eq .Format "png" }} selected {{ end }} value="png">PNG</option>
					<option {{ if eq .Format "jpeg" }} selected {{ end }} value="jpeg">JPEG</option>
					<option {{ if eq .Format "tif" }} selected {{ end }} value="tif">TIF</option>
				</select>
			</div>
			<div>
			<label for="resolution">解像度</label>
				<select id="resolution" name="resolution">
					<option {{ if eq .Resolution "150" }} selected {{ end }} value="150">150 dpi</option>
					<option {{ if eq .Resolution "200" }} selected {{ end }} value="200">200 dpi</option>
					<option {{ if eq .Resolution "300" }} selected {{ end }} value="300">300 dpi</option>
					<option {{ if eq .Resolution "600" }} selected {{ end }} value="600">600 dpi</option>
				</select>
			</div>
			<div>
				<label for="size">用紙サイズ</label>
				<select id="size" name="size">
					<option {{ if eq .Size "A4" }} selected {{ end }} value="A4">A4</option>
					<option {{ if eq .Size "A5v" }} selected {{ end }} value="A5v">A5縦</option>
					<option {{ if eq .Size "A5h" }} selected {{ end }} value="A5h">A5横</option>
					<option {{ if eq .Size "B5" }} selected {{ end }} value="B5">B5</option>
				</select>
			</div>
			<button type="submit">保存</button>
		</form>
		<button onclick="start()">スキャン</button>
	</body>
</html>
`
