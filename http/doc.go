package http

const INDEX_DOC = `
<!doctype html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, minimum-scale=1, initial-scale=1, user-scalable=yes">
  <script type="text/javascript" src="https://cdn.jsdelivr.net/npm/rapidoc@8.4.3/dist/rapidoc-min.js"></script>
  <link
    href="https://fonts.googleapis.com/css2?family=Open+Sans:wght@300;600&family=Roboto+Mono&display=swap"
    rel="stylesheet">
  <style>
    rapi-doc {
      width: 100%;
    }
  </style>
</head>

<body>
  <rapi-doc spec-url="http://127.0.0.1:9999/doc.json"
    allow-server-selection="false"
    allow-authentication="true"

    show-header="false"
    layout="column"
    render-style="focused"
    schema-style="tree"
    regular-font='Open Sans'
    mono-font="Roboto Mono"

    show-info="false"
    show-components="false"
    >
  </rapi-doc>
</body>

</html>
`
