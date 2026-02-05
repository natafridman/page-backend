package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
		json.NewEncoder(w).Encode(Response{Error: "Method not allowed"})
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Obtener el ID de la carpeta raíz desde variables de entorno o query params
	rootFolderID := r.URL.Query().Get("folderId")
	if rootFolderID == "" {
		// Puedes configurar esto en las variables de entorno de Vercel
		rootFolderID = getEnv("GOOGLE_DRIVE_FOLDER_ID", "")
	}

	if rootFolderID == "" {
		json.NewEncoder(w).Encode(Response{Error: "Folder ID is required"})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Obtener credenciales desde variable de entorno
	credentialsJSON := getEnv("GOOGLE_CREDENTIALS_JSON", "")
	if credentialsJSON == "" {
		json.NewEncoder(w).Encode(Response{Error: "Google credentials not configured"})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	srv, err := drive.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		json.NewEncoder(w).Encode(Response{Error: fmt.Sprintf("Unable to create Drive client: %v", err)})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	items, err := getItems(srv, rootFolderID)
	if err != nil {
		json.NewEncoder(w).Encode(Response{Error: err.Error()})
		w.WriteHeader(http.StatusInternalServerError)
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
	}

	// Listar todos los archivos en la carpeta del item
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)
	fileList, err := srv.Files.List().Q(query).Fields("files(id, name, mimeType, webContentLink, webViewLink)").Do()
	if err != nil {
		return item, fmt.Errorf("error listing files in folder: %v", err)
	}

	var metadataFileID string

	for _, file := range fileList.Files {
		// Si es el archivo metadata.txt
		if file.Name == "metadata.txt" {
			metadataFileID = file.Id
			continue
		}

		// Si es una imagen
		if isImage(file.MimeType) {
			// Usar webContentLink para descarga directa o webViewLink para vista
			imageURL := getImageURL(file.Id)
			item.ImageURLs = append(item.ImageURLs, imageURL)
		}
	}

	// Leer metadata.txt si existe
	if metadataFileID != "" {
		metadata, err := readMetadata(srv, metadataFileID)
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

func getImageURL(fileID string) string {
	// URL pública para ver/descargar la imagen
	// Requiere que los archivos sean públicos o que uses OAuth
	return fmt.Sprintf("https://drive.google.com/uc?export=view&id=%s", fileID)
}

func readMetadata(srv *drive.Service, fileID string) (map[string]string, error) {
	resp, err := srv.Files.Get(fileID).Download()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseMetadata(string(body)), nil
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

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
