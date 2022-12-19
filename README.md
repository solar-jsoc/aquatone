# AQUATONE

See [original docs](https://github.com/michenriksen/aquatone)

Добавлены ключи:

`-tar`: упаковывает результаты работы утилиты в архив report.tar.gz

`-out-file`: перенаправляет вывод утилиты в указанный файл, ***обязателен к использованию с ключом -tar***, иначе вывод не попадёт в архив

Пример использования:
```shell
aquatone [some other args] -out-file=aquatone.out.txt -tar
```