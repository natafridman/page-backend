# Google Drive Items API - Vercel Serverless Function

Este proyecto implementa un endpoint serverless en Go para Vercel que lee carpetas de Google Drive y devuelve información estructurada sobre items con imágenes y metadata.

## Estructura de Google Drive

```
Root Folder/
├── Item 1/
│   ├── metadata.txt
│   ├── imagen1.jpg
│   ├── imagen2.png
│   └── imagen3.jpg
├── Item 2/
│   ├── metadata.txt
│   ├── foto1.jpg
│   └── foto2.jpg
└── Item 3/
    ├── metadata.txt
    └── imagen.png
```

### Formato de metadata.txt

```
title: Mi Título
subtitle: Mi Subtítulo
description: Una descripción detallada del item
code: ABC123
```

## Configuración

### 1. Credenciales de Google Cloud

1. Ve a [Google Cloud Console](https://console.cloud.google.com/)
2. Crea un nuevo proyecto o selecciona uno existente
3. Habilita la API de Google Drive
4. Crea credenciales de tipo "Service Account"
5. Descarga el archivo JSON de credenciales

### 2. Permisos en Google Drive

1. Abre la carpeta raíz en Google Drive
2. Comparte la carpeta con el email del Service Account (está en el JSON de credenciales)
3. Dale permisos de "Viewer" o "Editor"
4. Copia el ID de la carpeta desde la URL:
   ```
   https://drive.google.com/drive/folders/[FOLDER_ID_AQUÍ]
   ```

### 3. Variables de Entorno en Vercel

Configura estas variables en tu proyecto de Vercel:

- `GOOGLE_CREDENTIALS_JSON`: El contenido completo del archivo JSON de credenciales (como string)
- `GOOGLE_DRIVE_FOLDER_ID`: El ID de tu carpeta raíz en Google Drive

Para configurar en Vercel:
```bash
vercel env add GOOGLE_CREDENTIALS_JSON
vercel env add GOOGLE_DRIVE_FOLDER_ID
```

## Deployment

```bash
# Instalar Vercel CLI
npm i -g vercel

# Deploy
vercel

# Deploy a producción
vercel --prod
```

## Uso del API

### Endpoint

```
GET /api/items
```

### Query Parameters (opcional)

- `folderId`: ID de la carpeta de Google Drive (si no usas variable de entorno)

### Ejemplo de petición

```bash
curl https://tu-proyecto.vercel.app/api/items
```

### Ejemplo de respuesta

```json
{
  "items": [
    {
      "title": "Producto A",
      "subtitle": "Categoría Premium",
      "description": "Descripción detallada del producto A",
      "code": "PROD-001",
      "imageUrls": [
        "https://drive.google.com/uc?export=view&id=abc123",
        "https://drive.google.com/uc?export=view&id=def456"
      ]
    },
    {
      "title": "Producto B",
      "subtitle": "Categoría Estándar",
      "description": "Descripción del producto B",
      "code": "PROD-002",
      "imageUrls": [
        "https://drive.google.com/uc?export=view&id=ghi789"
      ]
    }
  ]
}
```

## Estructura del Proyecto

```
.
├── api/
│   └── items.go          # Función serverless principal
├── go.mod                # Dependencias de Go
├── vercel.json           # Configuración de Vercel
└── README.md             # Este archivo
```

## Notas Importantes

1. **Permisos de archivos**: Las imágenes deben ser accesibles públicamente o el Service Account debe tener acceso
2. **Límites de Vercel**: 
   - Timeout máximo: 10 segundos (configurable según plan)
   - Memoria: 1024 MB (configurable)
3. **Formato de metadata.txt**: Debe usar el formato `key: value` en cada línea
4. **Imágenes soportadas**: JPEG, PNG, GIF, WebP, BMP

## Mejoras Sugeridas

- Cache de respuestas para mejorar performance
- Paginación para carpetas con muchos items
- Validación más robusta del metadata.txt
- Soporte para otros tipos de archivos
- Thumbnails optimizados de las imágenes
- Manejo de errores más detallado

## Troubleshooting

### Error: "Unable to create Drive client"
- Verifica que `GOOGLE_CREDENTIALS_JSON` esté configurado correctamente
- Asegúrate de que el JSON sea válido

### Error: "Folder ID is required"
- Configura `GOOGLE_DRIVE_FOLDER_ID` en Vercel
- O pasa `?folderId=XXX` en la URL

### Las imágenes no se muestran
- Verifica que los archivos sean públicos en Google Drive
- O que el Service Account tenga acceso a ellos
- Considera usar `webViewLink` en lugar de `webContentLink` si tienes problemas

### Timeout en la función
- Reduce el número de items en la carpeta
- Implementa paginación
- Aumenta el timeout en `vercel.json` (requiere plan Pro)
