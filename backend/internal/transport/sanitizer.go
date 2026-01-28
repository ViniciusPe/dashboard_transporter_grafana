package transport

func SanitizeDashboard(dashboard map[string]interface{}) map[string]interface{} {
	// Campos que NUNCA devem ir para outro ambiente
	delete(dashboard, "id")
	delete(dashboard, "version")

	// Opcional (recomendado):
	delete(dashboard, "uid") // decide depois se mantém ou não

	return dashboard
}
