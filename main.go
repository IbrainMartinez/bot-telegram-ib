package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
    "github.com/joho/godotenv"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// Estructuras para manejar el webhook de Telegram
type TelegramUpdate struct {
    UpdateID int     `json:"update_id"`
    Message  Message `json:"message"`
}

type Message struct {
    Chat Chat   `json:"chat"`
    Text string `json:"text"`
}

type Chat struct {
    ID int64 `json:"id"`
}

// Estructura para el documento de MongoDB
// Se ha eliminado el campo "_id" para que MongoDB lo genere automáticamente.
type MessageData struct {
    Message string    `bson:"message"`
    URL     string    `bson:"url"`
    Date    time.Time `bson:"date"`
}

var (
    bot         *tgbotapi.BotAPI
    mongoClient *mongo.Client
)

func main() {
    // Carga las variables de entorno
    if err := godotenv.Load(); err != nil {
        log.Println("Advertencia: No se pudo cargar el archivo .env. Asegúrate de que las variables de entorno están configuradas.")
    }

    // Obtén el token del bot de las variables de entorno
    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    if token == "" {
        log.Fatal("Error: La variable de entorno TELEGRAM_BOT_TOKEN no está configurada.")
    }

    // Obtén la URI de MongoDB del .env
    mongoURI := os.Getenv("MONGO_URI")
    if mongoURI == "" {
        log.Fatal("Error: La variable de entorno MONGO_URI no está configurada.")
    }

    // Inicializa el bot de Telegram
    var err error
    bot, err = tgbotapi.NewBotAPI(token)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Bot autorizado en la cuenta %s", bot.Self.UserName)

    // Inicializa la conexión a MongoDB
    clientOptions := options.Client().ApplyURI(mongoURI)
    mongoClient, err = mongo.Connect(context.TODO(), clientOptions)
    if err != nil {
        log.Fatal(err)
    }
    // Añade la desconexión al final del programa para una buena práctica
    defer mongoClient.Disconnect(context.TODO())

    // Verifica la conexión
    if err = mongoClient.Ping(context.TODO(), nil); err != nil {
        log.Fatal(err)
    }
    log.Println("Conectado exitosamente a MongoDB!")

    // Cambia el modo de Gin a "release" para producción
    gin.SetMode(gin.ReleaseMode)
    router := gin.Default()

    router.GET("/", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "Hello World from your Go Bot!"})
    })
    router.POST("/webhook", handleTelegramWebhook)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    log.Printf("Iniciando servidor en el puerto :%s", port)
    log.Fatal(router.Run(":" + port))
}

// handleTelegramWebhook es el handler para los mensajes de Telegram
func handleTelegramWebhook(c *gin.Context) {
    var update TelegramUpdate
    if err := c.BindJSON(&update); err != nil {
        log.Printf("Error al decodificar JSON: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "JSON no válido"})
        return
    }

    log.Printf("Nuevo mensaje del chat %d: %s", update.Message.Chat.ID, update.Message.Text)

    messageText := update.Message.Text
    // Utiliza una función para extraer la URL de cualquier parte del mensaje
    url := extractURL(messageText)

    if url != "" {
        // Crea el documento para guardar en MongoDB
        messageData := MessageData{
            Message: messageText,
            URL:     url,
            Date:    time.Now(),
        }

        // Define la colección y base de datos
        collection := mongoClient.Database("tu_nombre_de_base_de_datos").Collection("modelos3d")
        _, err := collection.InsertOne(context.TODO(), messageData)
        if err != nil {
            log.Printf("Error al guardar en MongoDB: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Error interno del servidor"})
            return
        }
        
        // Responde al usuario con el mensaje de confirmación
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, "¡Mensaje y enlace para modelo 3D recibidos y guardados con éxito!")
        bot.Send(msg)
    } else {
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Por favor, envía un enlace que comience con http:// o https:// para el modelo 3D.")
        bot.Send(msg)
    }

    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// extractURL extrae la primera URL de una cadena de texto
func extractURL(text string) string {
    parts := strings.Split(text, " ")
    for _, part := range parts {
        if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
            return part
        }
    }
    return ""
}