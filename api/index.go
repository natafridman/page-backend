package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type Item struct {
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"`
	Description string   `json:"description"`
	Code        string   `json:"code"`
	ImageURLs   []string `json:"imageUrls"`
	VideoURLs   []string `json:"videoUrls"`
}

type Response struct {
	Items []Item `json:"items"`
	Error string `json:"error,omitempty"`
}

// Handler es la función principal que maneja las peticiones en Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{Error: "Method not allowed"})
		return
	}

	// Obtener el ID de la carpeta raíz desde variables de entorno o query params
	rootFolderID := r.URL.Query().Get("folderId")
	if rootFolderID == "" {
		rootFolderID = os.Getenv("GOOGLE_DRIVE_FOLDER_ID")
	}

	if rootFolderID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "Folder ID is required"})
		return
	}

	// Obtener credenciales desde variable de entorno
	credentialsJSON := os.Getenv("GOOGLE_CREDENTIALS_JSON")
	if credentialsJSON == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: "Google credentials not configured"})
		return
	}

	ctx := context.Background()
	srv, err := drive.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: fmt.Sprintf("Unable to create Drive client: %v", err)})
		return
	}

	items, err := getItems(srv, rootFolderID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(Response{Items: items})
}

func getItems(srv *drive.Service, rootFolderID string) ([]Item, error) {
	var items []Item

	// Listar todas las carpetas dentro de la carpeta raíz
	query := fmt.Sprintf("'%s' in parents and mimeType='application/vnd.google-apps.folder' and trashed=false", rootFolderID)
	folderList, err := srv.Files.List().Q(query).Fields("files(id, name)").Do()
	if err != nil {
		return nil, fmt.Errorf("error listing folders: %v", err)
	}

	// Procesar cada carpeta (cada item)
	for _, folder := range folderList.Files {
		item, err := processItemFolder(srv, folder.Id, folder.Name)
		if err != nil {
			// Log error pero continuar con los demás items
			fmt.Printf("Error processing folder %s: %v\n", folder.Name, err)
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func processItemFolder(srv *drive.Service, folderID, folderName string) (Item, error) {
	item := Item{
		ImageURLs: []string{},
		VideoURLs: []string{},
	}

	// Listar todos los archivos en la carpeta del item
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)
	fileList, err := srv.Files.List().Q(query).Fields("files(id, name, mimeType, webContentLink, webViewLink)").Do()
	if err != nil {
		return item, fmt.Errorf("error listing files in folder: %v", err)
	}

	var metadataFileID string
	var metadataFileName string

	for _, file := range fileList.Files {
		// Si es el archivo metadata.txt o metadata.docx
		if file.Name == "metadata.txt" || file.Name == "metadata.docx" {
			metadataFileID = file.Id
			metadataFileName = file.Name
			continue
		}

		// Si es una imagen
		if isImage(file.MimeType) {
			imageURL := getImageURL(file.Id)
			item.ImageURLs = append(item.ImageURLs, imageURL)
		}

		// Si es un video
		if isVideo(file.MimeType) {
			videoURL := getVideoURL(file.Id)
			item.VideoURLs = append(item.VideoURLs, videoURL)
		}
	}

	// Leer metadata.txt o metadata.docx si existe
	if metadataFileID != "" {
		metadata, err := readMetadata(srv, metadataFileID, metadataFileName)
		if err != nil {
			return item, fmt.Errorf("error reading metadata: %v", err)
		}
		item.Title = metadata["title"]
		item.Subtitle = metadata["subtitle"]
		item.Description = metadata["description"]
		item.Code = metadata["code"]
	}

	return item, nil
}

func isImage(mimeType string) bool {
	imageTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	}
	for _, t := range imageTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

func isVideo(mimeType string) bool {
	videoTypes := []string{
		"video/mp4",
		"video/mpeg",
		"video/quicktime",
		"video/x-msvideo",
		"video/x-ms-wmv",
		"video/webm",
		"video/ogg",
		"video/3gpp",
		"video/x-flv",
	}
	for _, t := range videoTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

func getImageURL(fileID string) string {
	// URL pública para ver/descargar la imagen
	return fmt.Sprintf("https://drive.google.com/uc?export=view&id=%s", fileID)
}

func getVideoURL(fileID string) string {
	// URL para reproducir video desde Google Drive
	return fmt.Sprintf("https://drive.google.com/file/d/%s/preview", fileID)
}

func readMetadata(srv *drive.Service, fileID, fileName string) (map[string]string, error) {
	resp, err := srv.Files.Get(fileID).Download()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var content string
	
	// Si es un archivo .docx, usar pandoc para extraer el texto
	if strings.HasSuffix(strings.ToLower(fileName), ".docx") {
		// Guardar temporalmente el archivo
		tmpFile := fmt.Sprintf("/tmp/metadata_%s.docx", fileID)
		if err := os.WriteFile(tmpFile, body, 0644); err != nil {
			return nil, fmt.Errorf("error writing temp file: %v", err)
		}
		defer os.Remove(tmpFile)

		// Usar pandoc para extraer texto
		cmd := exec.Command("pandoc", tmpFile, "-t", "plain")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("error running pandoc: %v", err)
		}
		content = string(output)
	} else {
		// Es un archivo .txt
		content = string(body)
	}

	return parseMetadata(content), nil
}

func parseMetadata(content string) map[string]string {
	metadata := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			metadata[key] = value
		}
	}

	return metadata
}
