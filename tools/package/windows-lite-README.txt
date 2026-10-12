Feed AI для Windows x64 (облегчённая сборка)

Запуск
  1. Распакуйте архив.
  2. Запустите start.bat из папки feed-ai-win64-lite. Чтобы остановить
     сервер, закройте окно консоли.
  3. Откройте в браузере http://localhost:8080

Медиафайлы кладите в папку test-content. Сервер сканирует её при запуске,
поэтому файлы, добавленные вручную, появятся после перезапуска.

FFmpeg
  В эту сборку не входят FFmpeg и FFprobe. Без них нет постеров и
  длительности видео, превью кадров при перемотке и обложек аудио;
  остальное работает.

  Чтобы включить эти функции, скачайте
  https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip
  и положите ffmpeg.exe и ffprobe.exe из папки bin архива в
  tools\ffmpeg\windows-amd64\ рядом с server.exe.
  Также подойдёт FFmpeg, доступный через PATH.
