package main

import (
	"fmt"
	"github.com/juruen/rmapi/annotations"
	"github.com/juruen/rmapi/api"
	"github.com/juruen/rmapi/filetree"
	"github.com/juruen/rmapi/log"
	"github.com/juruen/rmapi/model"
	"github.com/juruen/rmapi/util"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sort"
)

type ViewHandler struct {
	Api *api.ApiCtx
}

type DownloadHandler struct {
	Api *api.ApiCtx
}

type Error struct {
	Text string
}

type ViewItem struct {
	Name      string
	Path      string
	Directory bool
}

type ViewData struct {
	Path  string
	Items []ViewItem
}

func ShowErrorPage(w http.ResponseWriter, err Error) {
	t, _ := template.ParseFiles("templates/error.html")
	t.Execute(w, err)
}

func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	docpath := r.URL.Path[len("/download/"):]
	node, _ := h.Api.Filetree.NodeByPath(docpath, h.Api.Filetree.Root())
	if node.IsFile() {
		err := preparePdf(h.Api, node)
		if err != nil {
			ShowErrorPage(w, Error{Text: fmt.Sprintf("Couldn't create PDF file for %s. %s",
				node.Name(), err.Error())})
			return
		}
		http.ServeFile(w, r, fmt.Sprintf("%s.pdf", node.Name()))
		defer os.Remove(fmt.Sprintf("%s.pdf", node.Name()))
	}
}

func (h *ViewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("templates/view.html")
	root := h.Api.Filetree.Root()
	docpath := r.URL.Path[len("/view/"):]
	if len(docpath) > 0 {
		node, _ := h.Api.Filetree.NodeByPath(docpath, root)
		root = node
	}
	var items []ViewItem
	for _, node := range root.Children {
		item := ViewItem{Name: node.Name(), Path: path.Clean(docpath), Directory: node.IsDirectory()}
		items = append(items, item)
	}
	if docpath == "" {
		docpath = "/"
	} else {
		item := ViewItem{Name: "Previous level", Path: "..", Directory: false}
		items = append(items, item)
	}
	sort.Slice(items, func(x, y int) bool {
		if items[x].Path == ".." {
			return true
		}
		return items[x].Name < items[y].Name
	})
	data := ViewData{
		Path:  docpath,
		Items: items,
	}
	t.Execute(w, data)
}

func preparePdf(api *api.ApiCtx, node *model.Node) error {
	api.FetchDocument(node.Document.ID, "tmp.zip")
	tmp, _ := ioutil.TempDir("", "rmapizip")
	err := util.Unzip("tmp.zip", tmp)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	defer os.Remove("tmp.zip")

	unzipDir := path.Join(tmp, node.Id())
	pdfName := fmt.Sprintf("%s.pdf", node.Name())
	generator := annotations.CreatePdfGenerator(unzipDir, pdfName)
	err = generator.Generate()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	log.InitLog()
	ctx := api.CreateApiCtx(api.AuthHttpCtx())
	filetree.CreateFileTreeCtx()

	viewHandler := ViewHandler{Api: ctx}
	http.HandleFunc("/view/", viewHandler.ServeHTTP)

	downloadHandler := DownloadHandler{Api: ctx}
	http.HandleFunc("/download/", downloadHandler.ServeHTTP)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/view/", 301)
	})

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css/"))))
	http.ListenAndServe(":8080", nil)
}
