#!/bin/sh

# Инициализация базы данных
sqlite3 /app/data/birthdays.db < /app/init_birthdays.sql

# Запуск бота
./birthday-bot 