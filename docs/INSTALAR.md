# Instalar Folio en la MacBook

## La forma fácil (recomendada)

Abre **Terminal** (Cmd + Espacio, escribe "Terminal") y pega:

```
curl -fsSL https://github.com/Saul-Punybz/folio-pr/releases/latest/download/instalar.sh | bash
```

Comprueba el chip, baja Folio sin la marca de cuarentena (Gatekeeper no sale), lo instala en Aplicaciones, lo deja arrancando solo al iniciar sesión y abre el navegador. La misma línea actualiza; los datos se quedan. Con el zip a mano: `bash instalar.sh ~/Downloads/Folio.zip`.

Luego sigue en **3. Crear su acceso**. Los pasos 1 y 2 son la forma manual.

Para hacerlo una sola vez, con la usuaria al lado. Toma unos 15 minutos más la primera búsqueda (unos 12 minutos que corren solos).

## Antes de empezar

- Una MacBook con chip de Apple (M1 o más nuevo). Folio no abre en Mac con Intel.
- El archivo `Folio.zip` (está en `build/Folio.zip` del proyecto). Pásalo por AirDrop, USB o Drive.
- Conexión a internet.
- Su sesión de Google abierta, si van a crear las Alertas de Google el mismo día.

## 1. Copiar la app

1. Abre `Folio.zip` con doble clic. Aparece **Folio**.
2. Arrastra **Folio** a la carpeta **Aplicaciones**. No la abras desde Descargas.
3. Borra el `.zip`.

## 2. Autorizarla en macOS (solo la primera vez)

Folio no tiene firma de Apple (para no pagar los $99 al año), así que macOS la bloquea la primera vez. Es normal.

1. En **Aplicaciones**, doble clic en **Folio**. Sale un aviso de que Apple no pudo verificar la app. Pulsa **Listo** (no "Mover a la papelera").
2. Abre **Ajustes del Sistema → Privacidad y seguridad**.
3. Baja hasta **Seguridad**. Verás un aviso de que se bloqueó **Folio**. Pulsa **Abrir de todos modos**.
4. Pon la contraseña de la Mac (o Touch ID).
5. Vuelve a abrir **Folio** y, si pregunta otra vez, pulsa **Abrir**.

Desde ahí abre normal, siempre.

**Si no abre** (el navegador no se abre después de un minuto): abre **Terminal** y pega esta línea, que quita la marca de "descargado de internet" solo a Folio:

```
xattr -dr com.apple.quarantine /Applications/Folio.app
```

Luego abre Folio otra vez.

## 3. Crear su acceso

1. Folio abre el navegador en `http://127.0.0.1:47615` y muestra **Crea tu acceso**.
2. Ella pone su correo y una contraseña de al menos 10 caracteres. Que la guarde en su gestor de contraseñas o en el llavero de la Mac.
3. Entra al **Inbox**.

Folio solo funciona dentro de esa Mac: nadie más en la red puede abrirlo.

## 4. La primera búsqueda

Al abrir, Folio sale solo a buscar:

- **Noticias** de Puerto Rico y de fuera sobre Puerto Rico: empiezan a llegar en uno o dos minutos.
- **Marcas** (Project Makers, Vimenti, Oasis 360, PSI): unos minutos.
- **Fondos** (subastas, RFP, grants): la búsqueda completa tarda unos 12 minutos.

No hay que hacer nada; puede seguir trabajando.

## 5. Alertas de Google (recomendado, 2 minutos por marca)

Es lo que más menciones en la web le trae a cada marca.

1. Entra a **google.com/alerts** con su cuenta de Google.
2. Escribe la búsqueda. Sugeridas:
   - `"Project Makers" Puerto Rico`
   - `Vimenti`
   - `"Oasis 360" OR Oasis360 Puerto Rico`
   - `"Platform for Social Impact" OR "Impact PR"`
   - `"Eduardo Carrera" PSI`
3. Pulsa **Mostrar opciones** y en **Enviar a** elige **Feed RSS**. (Esa opción solo aparece con la sesión de Google iniciada.)
4. Pulsa **Crear alerta**.
5. En la lista de alertas, pulsa el ícono **RSS** de esa alerta y copia la dirección.
6. En Folio: **Marcas →** la marca **→ Editar → Feeds de alertas**, pega la dirección y guarda.

## Uso diario

- **Abrirlo:** doble clic en Folio cada mañana. Se pone al día solo.
- **Que abra solo al encender la Mac** (opcional): **Ajustes del Sistema → General → Ítems de inicio → +** y elige Folio.
- **Cerrarlo:** **Ajustes** dentro de Folio **→ Cerrar Folio**. Si la Mac se apaga con Folio abierto, no pasa nada: se recupera solo al abrirlo.
- **Mientras esté abierto** busca noticias y menciones de las marcas cada 3 horas, y fondos cada 2 horas. Si la Mac estuvo apagada, busca al abrirlo.

## Dónde están sus datos

- Todo vive en esa Mac, en `~/Library/Application Support/Folio2` (noticias, marcas, fondos, PDF guardados).
- Folio hace un **respaldo diario** en `Folio2/respaldos` (solo mientras está abierto: a mediodía y al abrirlo si el último tiene más de 20 horas) y guarda los últimos 14.
- Para estar seguros de no perder nada, que ese disco entre en **Time Machine** o en su respaldo habitual.

## Actualizar Folio más adelante

1. Cierra Folio (**Ajustes → Cerrar Folio**).
2. Reemplaza **Folio** en Aplicaciones por la versión nueva.
3. Ábrelo. Sus datos se quedan; Folio actualiza la base solo al arrancar. Puede que macOS pida otra vez **Abrir de todos modos**.

## Si algo sale mal

| Qué pasa | Qué hacer |
|---|---|
| macOS dice que la app está dañada o que no se puede abrir | Paso 2. Si sigue, la línea de Terminal del paso 2 |
| Doble clic y no pasa nada | Abre el navegador en `http://127.0.0.1:47615`: puede que ya esté abierto |
| Aviso de que el puerto 47615 está ocupado | Reinicia la Mac y abre Folio otra vez |
| Olvidó la contraseña | Ver **Olvidó la contraseña** abajo |
| No llegan noticias | Revisa la conexión a internet y espera unos minutos; el registro está en `Folio2/logs/folio.log` |

### Olvidó la contraseña

1. Cierra Folio: **Ajustes → Cerrar Folio**.
2. Abre **Terminal** (Spotlight: escribe "Terminal").
3. Pega esta línea, con su correo y la contraseña nueva (mínimo 10 caracteres) dentro de las comillas, y pulsa Enter:

```
FOLIO_PASSWORD='contraseña-nueva-aquí' /Applications/Folio.app/Contents/MacOS/Folio user reset correo@ejemplo.com
```

4. Debe decir `contraseña cambiada para correo@ejemplo.com`. Si dice `no existe el usuario`, el correo está mal escrito; los correos registrados se ven con la misma línea cambiando `user reset correo@ejemplo.com` por `user list` (sin `FOLIO_PASSWORD`).
5. Abre Folio y entra con la contraseña nueva. Las sesiones que tuviera abiertas se cierran.

Si prefieres no escribir la contraseña en la línea, quita `FOLIO_PASSWORD='…'` y Folio la pide dos veces sin mostrarla.
