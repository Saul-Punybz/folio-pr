# Folio — sesión del 19 nov 2026

Resumen local de lo que se hizo hoy. El estado oficial sigue en `CONTINUAR.md`.

## Lo que se hizo

| Qué | Dónde | Estado |
|---|---|---|
| Instalador de una línea | `scripts/instalar.sh` | Hecho, en GitHub |
| Release **v2.0.0** con `Folio.zip` + `instalar.sh` | github.com/Saul-Punybz/folio-pr/releases/tag/v2.0.0 | Publicado, marcado como el último |
| Guía de instalación con la línea como forma principal | `docs/INSTALAR.md` | Hecho |
| README reescrito con lo que Folio hace hoy | `README.md` | Hecho |
| Descripción y etiquetas del repo en GitHub (fuera «AI-powered», Ollama) | Ajustes del repo | Hecho |
| Correo con la línea para la usuaria (bpaganrosa@gmail.com) | Gmail → Borradores | **Falta que Saul lo envíe** |

## La línea de instalación

```
curl -fsSL https://github.com/Saul-Punybz/folio-pr/releases/latest/download/instalar.sh | bash
```

Qué hace:

1. Revisa que la Mac tenga chip de Apple; si es Intel, para.
2. Baja `Folio.zip` con curl. curl no le pone la marca de cuarentena, así que Gatekeeper no sale.
3. Si Folio está abierto, lo cierra bien (SIGTERM, igual que «Cerrar Folio») y reemplaza la app. Los datos en `Folio2` no se tocan.
4. Lo copia a `/Applications`, o a `~/Applications` si el usuario no es administrador. Sin sudo.
5. Crea el LaunchAgent `pr.folio.app`: arranca al iniciar sesión, en segundo plano; se reinicia si se cae; «Cerrar Folio» lo deja cerrado.
6. Espera a que responda `http://127.0.0.1:47615` y abre el navegador.

Con el zip a mano (AirDrop/USB): `bash instalar.sh ~/Downloads/Folio.zip`.
Para probar sin tocar la Mac real: `FOLIO_DEST=<carpeta> FOLIO_NO_START=1`.

## Verificado

- La línea corrida desde el release a una carpeta de prueba: baja, instala, la firma pasa, cero atributos de cuarentena.
- `Folio.zip` del release = `build/Folio.zip` local (sha256 `b61a147a…`), armado el 16 nov 13:51, después del último commit de código.
- `instalar.sh` del release = el del repo (sha256 `9949ae2f…`).
- `main` y `obra-1` en el mismo commit en GitHub.

## Sin verificar

- El arranque automático (LaunchAgent) y la espera a que responda: no se corrieron en la Mac de Saul para no tocar su Folio. La primera prueba completa es en la MacBook de ella.

## Pendiente

1. Enviar el borrador del correo.
2. Que ella corra la línea; si falla, que mande foto de Terminal.
3. Decidir si se borra el release viejo **v1.0.0** (marzo, app con Ollama en `.dmg`). No afecta la instalación; el código de marzo sigue en la rama `estado-marzo-2026`.
4. Lo de antes, sin cambio: Alertas de Google por marca, misión de la organización para Grants.gov, Bárbara Rivera Batista y Lucas Arzola como personas clave, mención falsa de PSI, errata «projctmakerspr».
5. Siguiente obra cuando ella lo haya usado 1–2 semanas: Escritos (con carpetas y citas en Guardados).

## Para publicar una versión nueva

```bash
make mac
gh release create vX.Y.Z -R Saul-Punybz/folio-pr --target main --latest build/Folio.zip scripts/instalar.sh
```

Ella actualiza con la misma línea.
