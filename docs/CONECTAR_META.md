> **BORRADOR INCOMPLETO (16 nov 2026).** Se detuvo a medias porque la conexión con Meta quedó para el futuro. Revisar contra la documentación de Meta antes de usarla.

# Conectar Folio con Facebook e Instagram (Meta)

Guía para una persona no técnica. Al terminar, Folio podrá leer las publicaciones y los comentarios de las páginas de Facebook y las cuentas de Instagram de Project Makers, Oasis360, Vimenti y Platform for Social Impact (PSI), y las publicaciones de otras personas que las mencionan o las etiquetan.

Revisada con la documentación oficial de Meta el 16 de noviembre de 2026. Meta cambia sus pantallas con frecuencia: si algo se ve distinto a lo que dice aquí, busca la opción con el nombre parecido. Las partes que la documentación no confirma están marcadas con **[NO CONFIRMADO]**.

---

## 1. Antes de empezar

**Quién lo hace:** una sola persona que sea administradora, con acceso total, de las páginas de Facebook de las cuatro marcas. Esa persona crea la app de Meta y es la única que la va a usar. La app se queda en «modo Desarrollo»: no se publica ni la usa nadie más.

**Ten a mano:**
- Tu cuenta personal de Facebook (la que administra las páginas).
- El usuario y la contraseña de los cuatro Instagram.
- Un correo de contacto de la organización.
- Folio abierto en la Mac donde corre (`http://127.0.0.1:47615`).
- Para los hashtags (paso 7, opcional): los documentos legales de la organización.

**Cuánto toma:** de 45 a 90 minutos para los pasos 2 al 6. El paso 7 puede tomar semanas y no es obligatorio.

---

## 2. Pasar cada Instagram a cuenta profesional y vincularlo a su página

Meta exige que cada Instagram sea una cuenta **profesional** (empresa o creador) y que esté **vinculada a una página de Facebook**. Una cuenta personal de Instagram no funciona. Repite esto con las cuatro marcas.

**En la app de Instagram (cuenta de la marca):**
1. Entra a la cuenta de la marca. Toca tu foto de perfil, luego las tres rayitas, y entra a **Configuración y actividad**.
2. Busca **Tipo de cuenta y herramientas** y toca **Cambiar a cuenta profesional**. Si ya dice «Cambiar tipo de cuenta», ya es profesional.
3. Elige **Empresa** (o **Creador**) y la categoría que mejor describa la marca.
4. Si Instagram te pide conectar una página de Facebook, hazlo ahí mismo. Si no, sigue con el paso 5.
5. Toca **Editar perfil**. En «Información pública de la empresa» busca **Página**, toca **Conectar o crear**, y escoge la página de la marca. Si te pide configurar el **Centro de cuentas**, sigue las pantallas e inicia sesión con tu Facebook.

**Alternativa desde Facebook:** abre la página de la marca, ve a **Configuración**, luego **Cuentas vinculadas** (o «Instagram»), y pulsa **Conectar cuenta**.

**Cosas que pasan:**
- Solo se puede vincular **una** página de Facebook por cada cuenta de Instagram.
- Si Instagram te pide iniciar sesión en la cuenta de Facebook de *otra* persona, es porque esa persona ya administra ese Instagram en el Centro de cuentas. Pídele que lo vincule, o que te dé acceso.
- Si la página no aparece en la lista, es porque tu Facebook no tiene acceso de administrador a ella.

**Cómo comprobar que quedó vinculado:** en la página de Facebook, **Configuración > Cuentas vinculadas** debe mostrar el Instagram de la marca. En Instagram, **Editar perfil** debe mostrar el nombre de la página junto a «Página».

*Nota: estos pasos salen de guías de ayuda que describen los menús actuales de Instagram. La página oficial de Meta (help.instagram.com) no se pudo leer completa, así que los nombres exactos de los menús pueden variar un poco.*

---

## 3. Crear la cuenta de desarrollador y la app

1. Entra a https://developers.facebook.com con tu Facebook. Si es la primera vez, te pide **registrarte como desarrollador**: acepta los términos y verifica tu correo o teléfono.
2. Ve a https://developers.facebook.com/apps/creation/ (o pulsa **Crear app**).
3. **Detalles de la app:** nombre `Folio [nombre de la organización]` y el correo de contacto de la organización. Pulsa **Siguiente**.
4. **Casos de uso:** escoge **Manage everything on your Page** («Administrar todo en tu página»). Es el caso de uso oficial de Meta para publicar, moderar publicaciones y comentarios, y ver estadísticas de una página. Este caso agrega solos `pages_show_list`, `business_management` y `public_profile`, que no se pueden quitar.
   - Para Instagram no hace falta otro caso de uso: los permisos de Instagram se agregan en el paso 4 dentro de ese mismo caso.
   - **[NO CONFIRMADO]** La documentación no dice con esas palabras que este caso de uso lleve «Facebook Login for Business» a la app, aunque sí dice que las apps nuevas de tipo Negocio (Business) con un caso de uso compatible lo reciben automáticamente. Si al entrar al panel no ves «Facebook Login for Business» en el menú de la izquierda, vuelve a crear la app y escoge **Other** («Otro») y luego el tipo **Business**. Meta advierte que el tipo de app no se puede cambiar después.
5. **Negocio:** escoge «No quiero conectar un portafolio comercial todavía», o el portafolio de la organización si ya lo tienen (si ya es verificado, mejor para el paso 7). Pulsa **Siguiente**.
6. **Requisitos:** léelos y pulsa **Siguiente**. Luego **Ir al panel** (Go to dashboard).

Debes ver el **panel de la app** con el nombre que le pusiste y, arriba, el interruptor del modo de la app en **Desarrollo**. Déjalo así.

---

## 4. Configurar el inicio de sesión

### 4a. El URI de redirección

Es la dirección a la que Facebook te devuelve después de iniciar sesión. En Folio es:

```
http://localhost:47615/api/meta/callback
```

1. En el menú izquierdo del panel, abre **Facebook Login for Business** > **Settings** (Configuración).
2. En **Valid OAuth Redirect URIs** pega el URI de arriba, exactamente igual (puerto y ruta incluidos, sin nada extra al final). Guarda.
3. Si ves **Use Strict Mode for Redirect URIs**, déjalo en Sí. Eso significa que el URI debe coincidir exacto.

**[NO CONFIRMADO] ¿Meta permite `http://localhost`?** La documentación oficial que revisé no dice nada sobre `localhost` ni sobre `http` en modo Desarrollo. Lo único que confirma es que el URI debe coincidir exactamente con uno de la lista. Los foros y respuestas de la comunidad se contradicen (algunos dicen que sí funciona, otros que lo rechaza y hay que usar un nombre de dominio). **Hay que probarlo.** Si Meta rechaza guardar o da el error «URL bloqueada» (ver paso 8):
- Primera alternativa, la más simple: usar `http://127.0.0.1:47615/api/meta/callback` en vez de `localhost` (y que Folio use ese mismo URI). No está confirmado oficialmente tampoco.
- Segunda alternativa: una dirección `https` pública que apunte a Folio (por ejemplo un túnel). Es más complicado y deberían hacerlo con ayuda técnica.
Avísale a quien construye Folio qué te dijo Meta.

### 4b. La configuración y los permisos

Facebook Login for Business no usa una lista de permisos suelta: usa una **configuración** que agrupa los permisos.

1. En **Facebook Login for Business** > **Configurations** (Configuraciones), pulsa **+ Create configuration** (Crear configuración).
2. Ponle un nombre, por ejemplo `Folio lectura`. Como tipo de token, escoge **User access token** (token de acceso de usuario).
3. Escoge los permisos de esta tabla. Todos siguen existiendo con estos nombres en la documentación actual de Meta.

| Permiso | Para qué sirve (en una línea) |
|---|---|
| `pages_show_list` | Ver la lista de páginas que administras, para que elijas cuáles conectar. |
| `pages_read_engagement` | Leer lo que publica la página y sus datos (seguidores, estadísticas). |
| `pages_read_user_content` | Leer lo que escribe el público en la página: publicaciones de visitantes y comentarios. |
| `instagram_basic` | Leer el perfil y las publicaciones de la cuenta de Instagram de empresa. |
| `instagram_manage_comments` | Leer los comentarios de Instagram y las publicaciones públicas donde la cuenta es mencionada o etiquetada. (También permite responder o borrar comentarios; Folio solo debe leer.) |
| `business_management` | Leer los activos del portafolio comercial (las páginas que pertenecen a un portafolio). |

4. Pulsa **Create**. Meta te da un **Configuration ID**: cópialo y guárdalo, Folio podría pedirlo.

**Importante:** en Facebook Login for Business, la persona que inicia sesión tiene que aceptar **todos** los permisos o la app no recibe ninguno.

**[NO CONFIRMADO] Menciones y etiquetas en modo Desarrollo.** La documentación dice que las apps nuevas tienen «Acceso estándar», pensado para personas con rol en la app (tú, que la creaste) y que no requiere App Review para ese uso. También dice que el acceso estándar limita datos y que algunas funciones podrían no andar hasta tener «Acceso avanzado». Los puntos de lectura de menciones (`mentioned_media`) y etiquetas (`tags`) piden `instagram_basic`, `instagram_manage_comments` y `pages_read_engagement`, pero **ninguna de sus páginas dice si funcionan sin App Review cuando las cuentas son tuyas**. Lo razonable es que sí (las cuentas las administras tú), pero hay que probarlo con una mención real. Además, Meta no manda avisos de menciones de cuentas privadas.

---

## 5. Copiar el App ID y el App Secret y pegarlos en Folio

1. En el panel de la app, ve a **Configuración de la app > Básica** (App settings > Basic).
2. Copia el **App ID** (un número largo).
3. Junto a **App Secret**, pulsa **Mostrar**. Meta te pide tu contraseña de Facebook. Copia el valor.
4. Abre Folio, entra a **Ajustes > Conectar con Meta**, y pega el App ID y el App Secret. Guarda.

> **ADVERTENCIA:** el **App Secret es como una contraseña**. Quien lo tenga puede hacerse pasar por tu app. **No lo mandes por correo, chat ni a nadie.** Solo se pega en Folio, en tu computadora. Si crees que se filtró, en la misma pantalla de Meta puedes **restablecerlo** y pegar el nuevo en Folio.

---

## 6. Conectar

1. En Folio, **Ajustes > Conectar con Meta**, pulsa **Conectar con Meta**.
2. Se abre el inicio de sesión de Facebook. Entra con la cuenta que administra las páginas.
3. Facebook te mostrará los permisos que pide la app. Revísalos y continúa.
4. Te pide **elegir las páginas** de Facebook y las **cuentas de Instagram** que la app podrá ver. Marca las de las cuatro marcas: Project Makers, Oasis360, Vimenti y PSI. Si no te deja marcar todas, mira el paso 8.
5. Pulsa **Guardar / Listo**. Facebook te regresa a Folio.

Debes ver en Folio las cuatro páginas y sus cuatro Instagram como conectados. Si faltan, repite el paso 6 y marca las que faltan.

---

## 7. (Opcional) Hashtags: verificación de negocio y App Review

Solo hace falta si quieres que Folio busque hashtags de Instagram. Para eso Meta exige que la app tenga la función **Instagram Public Content Access** y el permiso `instagram_basic` aprobados por **App Review** (revisión de Meta). Ese «Acceso avanzado» además requiere la **verificación de negocio**.

**Límite:** cada cuenta de Instagram de empresa puede consultar **30 hashtags distintos cada 7 días**. Repetir el mismo hashtag en ese tiempo no cuenta de nuevo. No incluye Stories ni emojis, y no puedes comentar en lo que encuentres.

**Verificación de negocio (dónde):** en https://business.facebook.com > **Configuración del negocio > Centro de seguridad** > **Verificación**. (En el panel de la app, **App Review** te lleva ahí si hace falta.) Según la ayuda de Meta, el proceso incluye documentos aceptados para organizaciones sin fines de lucro, pero la página oficial no se pudo leer completa.

**Documentos (según fuentes de terceros que citan las reglas de Meta; confirma en la ayuda oficial de tu país):**
- Tienen que probar el **nombre legal** de la organización y su **dirección o teléfono**. Los nombres tienen que coincidir exactamente con lo que escribas en la información del negocio.
- Lo más usual: certificado de incorporación o registro de la entidad (por ejemplo del Departamento de Estado de PR), o la determinación de exención del IRS / Hacienda para entidades sin fines de lucro. **[NO CONFIRMADO]** que Meta acepte específicamente la carta del IRS.
- Una factura de luz o agua solo prueba dirección o teléfono, no el nombre. Las planillas que uno mismo radica no se aceptan.

**Cuánto tarda:** **[NO CONFIRMADO]** Meta no publica un tiempo. Cuenta con días o semanas, y con que puedan pedirte documentos de nuevo.

**App Review del permiso (dónde):** panel de la app > **Review > App Review** (o **Revisión de la app > Permisos y funciones**), busca **Instagram Public Content Access** y pulsa **Request**. Necesitarás: reglas de privacidad (URL), un video corto que muestre cómo Folio usa la función, e instrucciones para el revisor. Meta pide que confirmes que usas el permiso solo para lo permitido.

**Texto sugerido (inglés, honesto):**

> Folio is an internal tool used by [Organization name], a nonprofit in Puerto Rico, for its own four brands (Project Makers, Oasis360, Vimenti and Platform for Social Impact). We use Instagram Public Content Access with Hashtag Search to understand public sentiment about our own brands: we search a small set of branded hashtags (well under 30 per week) and read the public posts that use them, so our team can see how people talk about our programs. We do not publish, comment, message, or collect personal data to profile individuals, and we do not sell or share the data. The app is used only by the account administrator, who owns or manages all connected Facebook Pages and Instagram accounts. Data is stored locally on that user's computer.

**La aprobación no está garantizada.** Meta puede rechazarla o pedir cambios. Una app de uso interno, sin pantalla pública, a veces es difícil de revisar para ellos; la documentación dice que si el revisor no puede probar tu app (por ejemplo no tiene interfaz que pueda abrir), puedes pedir revisión solo de `instagram_basic` y `instagram_manage_comments`. Lo demás de esta guía funciona sin esto.

---

## 8. Si algo sale mal

- **La página no aparece al conectar.** Tu Facebook no es administrador con acceso total de esa página, o al iniciar sesión no la marcaste. Revisa en la página: **Configuración > Acceso a la página**. Luego repite el paso 6 y marca todas las páginas. Si Facebook ya no te deja marcar, quita la app (paso 9) y conecta de nuevo.
- **El Instagram no aparece.** Casi siempre es porque no es cuenta profesional o no está vinculado a la página. Repite el paso 2 y comprueba el vínculo. También puede estar vinculado a *otra* página. Solo se permite una.
- **El permiso venció / Folio dice «sesión expirada».** El acceso que da Facebook caduca. Entra a Folio > **Ajustes > Conectar con Meta** y pulsa **Conectar** otra vez. **[NO CONFIRMADO]** El tiempo exacto de duración depende de lo que escojas al crear la configuración (la página de Meta permite elegir la expiración del token); no lo verifiqué.
- **«URL bloqueada» o «URL no permitida» (URL blocked / redirect URI isn't whitelisted).** El URI de redirección no coincide exacto con el de **Valid OAuth Redirect URIs**. Revisa que sea idéntico, con el puerto `47615`, sin diagonal extra al final. Si sigue igual, ver las alternativas del paso 4a.
- **«App no disponible» (App not active / not available).** Casi siempre es porque inicias sesión con una cuenta que **no tiene rol en la app**. Entra con la cuenta que la creó, o añade la otra cuenta en **Roles de la app** (App roles) como administrador o desarrollador. Verifica también que la app siga en modo Desarrollo y no esté desactivada.
- **«Dado que la app no tiene permisos» / Folio no ve comentarios.** Te faltó un permiso en la configuración o no aceptaste todos al iniciar. Los permisos son todo o nada: revisa el paso 4b y vuelve a conectar.

---

## 9. Cómo desconectar

**En Folio:** **Ajustes > Conectar con Meta > Desconectar**. Esto borra lo guardado en tu computadora.

**En Facebook (para que Meta también quite el permiso):**
1. Entra a tu Facebook y ve a **Configuración y privacidad > Configuración**.
2. Busca **Integraciones comerciales** (Business Integrations). Enlace directo: https://www.facebook.com/settings?tab=business_tools
3. Selecciona la app `Folio [organización]` y pulsa **Quitar** (Remove). Confirma.
4. Solo puede quitarla la cuenta que hizo la conexión.

Si ya no vas a usar la app, además puedes eliminarla en https://developers.facebook.com/apps (Configuración de la app > Básica > Eliminar app).

---

## Fuentes consultadas (16 de noviembre de 2026)

- https://developers.facebook.com/docs/facebook-login/facebook-login-for-business
- https://developers.facebook.com/documentation/development/create-an-app
- https://developers.facebook.com/documentation/pages-api/create-an-app
- https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/overview
- https://developers.facebook.com/docs/instagram-platform/instagram-api-with-facebook-login/hashtag-search
- https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/tags
- https://developers.facebook.com/docs/instagram-api/reference/ig-user/mentioned_media/
- https://developers.facebook.com/docs/permissions
- https://developers.facebook.com/docs/facebook-login/security
- https://developers.facebook.com/blog/post/2017/12/18/strict-uri-matching/
- https://www.facebook.com/business/help/2058515294227817 (solo se leyó el índice)
- https://help.instagram.com/502981923235522 (solo se leyó el título; los pasos vienen de guías de terceros)
- Guías de terceros usadas solo como apoyo y marcadas arriba: Metricool, Leadsie, Telnyx, 360dialog (verificación de negocio); DealerRater, Constant Contact (Integraciones comerciales).
