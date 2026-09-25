package main

// Textos originales de los archivos de práctica. Imitan la estructura de los del curso, no su contenido.

const textArchivo = `Este es un archivo de práctica.
Puedes leerlo, copiarlo, moverlo o borrarlo sin miedo:
cada ejercicio empieza con una copia nueva.
`

const textDewey = `Melvil Dewey fue un bibliotecario estadounidense del siglo XIX.
En 1876 propuso una forma de ordenar los libros por tema usando números.
La idea es sencilla: el conocimiento se divide en diez grandes clases.
000 es para obras generales y computación.
100 es para filosofía y psicología.
200 es para religión.
300 es para ciencias sociales.
400 es para lenguas.
500 es para ciencias naturales y matemáticas.
600 es para tecnología y ciencias aplicadas.
700 es para artes y recreación.
800 es para literatura.
900 es para historia y geografía.
Cada clase se divide en diez, y cada división otra vez en diez.
Así, un número como 005.43 lleva de lo general a lo particular,
igual que una ruta como /home/alumno/taller lleva de la raíz a un directorio.
`

const textFrases = `Nadie nace sabiendo usar la terminal.
Leer el manual no es rendirse, es aprender.
Un comando a la vez, una opción a la vez.
El error de hoy es el truco de mañana.
Si no sabes dónde estás, pregunta con pwd.
Antes de borrar, mira dos veces.
La paciencia también es una herramienta.
Lo que se automatiza una vez se agradece mil veces.
Equivocarse en un sandbox es gratis.
Todo es un archivo, hasta las dudas.
La tecla Tab es tu mejor amiga.
Quien entiende sus herramientas trabaja menos y mejor.
`

const textEje1 = `# Ejercicio 1

Notas para practicar comandos básicos.

- Crear directorios con mkdir
- Crear archivos vacíos con touch
- Copiar con cp y mover con mv
- Borrar con rm (¡con cuidado!)
`

const textEjeBackup = `2024-03-04 09:00:01 INFO  inicio del respaldo
2024-03-04 09:00:02 INFO  copiando taller/archivo.txt
2024-03-04 09:00:02 INFO  copiando taller/dia-1/Dewey.txt
2024-03-04 09:00:03 WARN  taller/dia-1/frases.txt cambió durante la copia
2024-03-04 09:00:03 INFO  copiando taller/ejemplo.txt
2024-03-04 09:00:04 ERROR no se pudo leer taller/privado.txt: permiso denegado
2024-03-04 09:00:05 INFO  respaldo terminado con 1 error y 1 advertencia
`

const textEjeDatos = `nombre edad ciudad
Ana 21 Iztapalapa
Luis 19 Coyoacán
Marta 23 Tlalpan
Jorge 20 Iztapalapa
Sofía 22 Xochimilco
`

const textEjemplo = `Hola, soy un archivo de ejemplo.
Tengo varias líneas para que practiques con cat y less.
Esta es la tercera línea.
Y esta es la última.
`

const textEjePractica = `Práctica de comandos básicos
1. Averigua en qué directorio estás.
2. Lista el contenido del directorio, incluidos los archivos ocultos.
3. Crea un directorio llamado pruebas.
4. Copia este archivo dentro de pruebas.
`

const textEjes = `Un eje es una línea de referencia.
En el plano cartesiano hay dos ejes: x (horizontal) e y (vertical).
En el espacio se agrega un tercero: z.
La Tierra gira sobre su propio eje una vez al día.
`

const textNotasEje1 = `Notas del ejercicio 1
- mkdir -p crea también los directorios intermedios.
- rmdir solo borra directorios vacíos.
- rm -r borra un directorio con todo su contenido.
`

const textOculto = `¡Me encontraste!
Los archivos cuyo nombre empieza con punto están ocultos:
ls no los muestra, pero ls -a sí.
`

const textPoesia = `En la terminal oscura parpadea un cursor,
paciente como un faro que espera a su marinero.
Escribo ls y el mundo se despliega en columnas,
escribo cd y el camino cambia de dueño.

Los archivos duermen en su árbol de ramas,
cada uno con su nombre y su lugar exacto;
la raíz los sostiene a todos en silencio,
desde / hasta el último rincón de mi home.

Un pipe une dos voces en una sola frase,
lo que dice la primera lo escucha la segunda.
Y al final, cuando todo sale bien,
el prompt regresa, tranquilo, a esperarme.
`
