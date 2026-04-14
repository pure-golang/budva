package graph

import "net/http"

// PlaygroundHandler возвращает HTML-страницу GraphQL Playground.
func PlaygroundHandler(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		//nolint:gosec // Статический HTML, не пользовательский ввод
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css"/>
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root"></div>
  <script>
    window.addEventListener('load', function() {
      GraphQLPlayground.init(document.getElementById('root'), { endpoint: '` + endpoint + `' })
    })
  </script>
</body>
</html>`))
	}
}
