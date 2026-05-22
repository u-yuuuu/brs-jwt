import csv
import sqlite3
import hashlib
import sys
import os

def hash_password(password):
    """Функция хеширования пароля"""
    return hashlib.sha256(password.encode()).hexdigest()

def setup_database(db_file):
    """Создает все необходимые таблицы в базе данных"""
    conn = sqlite3.connect(db_file)
    cursor = conn.cursor()
    
    tables_sql = [
        """CREATE TABLE IF NOT EXISTS usertypes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE
        );""",
        
        """CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            type_id INTEGER REFERENCES usertypes (id),
            username TEXT UNIQUE,
            password TEXT NOT NULL
        );""",
        
        """CREATE TABLE IF NOT EXISTS subjects (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE
        );""",
        
        """CREATE TABLE IF NOT EXISTS courses (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            subject_id INTEGER REFERENCES subjects (id)
        );""",
        
        """CREATE TABLE IF NOT EXISTS groups (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE
        );""",
        
        """CREATE TABLE IF NOT EXISTS student_in_group (
            user_id INTEGER REFERENCES users (id) UNIQUE,
            group_id INTEGER REFERENCES groups (id)
        );""",
        
        """CREATE TABLE IF NOT EXISTS course_admins (
            user_id INTEGER REFERENCES users (id),
            course_id INTEGER REFERENCES courses (id)
        );""",
        
        """CREATE TABLE IF NOT EXISTS subject_attributes (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            subject_id INTEGER REFERENCES subjects (id),
            name TEXT,
            max_points REAL
        );""",
        
        """CREATE TABLE IF NOT EXISTS students_points (
            user_id INTEGER REFERENCES users (id),
            course_id INTEGER REFERENCES courses (id),
            attribute_id INTEGER REFERENCES subject_attributes(id),
            point REAL
        );""",
        
        """CREATE TABLE IF NOT EXISTS course_attendees (
            course_id INTEGER REFERENCES courses (id),
            group_id INTEGER REFERENCES groups (id)
        );""",
        
        """CREATE TABLE IF NOT EXISTS users_info (
            user_id INTEGER REFERENCES users (id) UNIQUE,
            last_name TEXT,
            first_name TEXT
        );"""
    ]
    
    for sql in tables_sql:
        cursor.execute(sql)
    
    # Добавляем стандартные типы пользователей
    default_types = ['Student', 'Professor', 'Admin']
    for type_name in default_types:
        cursor.execute("INSERT OR IGNORE INTO usertypes (name) VALUES (?)", (type_name,))
    
    conn.commit()
    conn.close()
    print(f"База данных {db_file} успешно настроена")

def import_users(csv_file, db_file):
    """Импортирует пользователей из CSV файла"""
    conn = sqlite3.connect(db_file)
    cursor = conn.cursor()
    
    with open(csv_file, 'r', encoding='utf-8') as file:
        csv_reader = csv.reader(file)
        
        for row_num, row in enumerate(csv_reader, 1):
            if len(row) < 5:
                print(f"Ошибка в строке {row_num}: неверное количество колонок ({len(row)} вместо минимум 5)")
                continue
            
            username = row[0]
            password = row[1]
            user_type = row[2]
            first_name = row[3]
            last_name = row[4]
            group_name = row[5] if len(row) > 5 else None
            
            try:
                # Получаем ID типа пользователя
                cursor.execute("SELECT id FROM usertypes WHERE name = ?", (user_type,))
                type_result = cursor.fetchone()
                
                if not type_result:
                    print(f"Ошибка в строке {row_num}: тип пользователя '{user_type}' не найден")
                    continue
                
                type_id = type_result[0]
                
                # Хешируем пароль
                hashed_password = hash_password(password)
                
                # Вставляем пользователя
                cursor.execute("""
                    INSERT OR REPLACE INTO users (type_id, username, password)
                    VALUES (?, ?, ?)
                """, (type_id, username, hashed_password))
                
                user_id = cursor.lastrowid
                
                # Добавляем информацию о пользователе
                cursor.execute("""
                    INSERT OR REPLACE INTO users_info (user_id, last_name, first_name)
                    VALUES (?, ?, ?)
                """, (user_id, last_name, first_name))
                
                # Если это студент и указана группа, добавляем в группу
                if user_type == 'Student' and group_name:
                    # Получаем или создаем группу
                    cursor.execute("INSERT OR IGNORE INTO groups (name) VALUES (?)", (group_name,))
                    cursor.execute("SELECT id FROM groups WHERE name = ?", (group_name,))
                    group_id = cursor.fetchone()[0]
                    
                    # Добавляем студента в группу
                    cursor.execute("""
                        INSERT OR REPLACE INTO student_in_group (user_id, group_id)
                        VALUES (?, ?)
                    """, (user_id, group_id))
                
                print(f"Добавлен пользователь: {username} ({first_name} {last_name})")
                
            except Exception as e:
                print(f"Ошибка в строке {row_num}: {e}")
    
    conn.commit()
    conn.close()
    print("Импорт пользователей завершен")

def import_subjects(csv_file, db_file):
    """Импортирует предметы и их атрибуты из CSV файла"""
    conn = sqlite3.connect(db_file)
    cursor = conn.cursor()
    
    with open(csv_file, 'r', encoding='utf-8') as file:
        csv_reader = csv.reader(file)
        
        for row_num, row in enumerate(csv_reader, 1):
            if len(row) < 1:
                continue
                
            subject_name = row[0]
            attributes = row[1:]  # Остальные элементы - атрибуты
            
            try:
                # Добавляем предмет
                cursor.execute("INSERT OR IGNORE INTO subjects (name) VALUES (?)", (subject_name,))
                cursor.execute("SELECT id FROM subjects WHERE name = ?", (subject_name,))
                subject_id = cursor.fetchone()[0]
                
                # Обрабатываем атрибуты (название, макс. баллы, название, макс. баллы...)
                for i in range(0, len(attributes), 2):
                    if i + 1 < len(attributes):
                        attr_name = attributes[i]
                        max_points = float(attributes[i + 1])
                        
                        cursor.execute("""
                            INSERT OR REPLACE INTO subject_attributes (subject_id, name, max_points)
                            VALUES (?, ?, ?)
                        """, (subject_id, attr_name, max_points))
                
                print(f"Добавлен предмет: {subject_name} с {len(attributes)//2} атрибутами")
                
            except Exception as e:
                print(f"Ошибка в строке {row_num}: {e}")
    
    conn.commit()
    conn.close()
    print("Импорт предметов завершен")

def import_courses(csv_file, db_file):
    """Импортирует курсы из CSV файла"""
    conn = sqlite3.connect(db_file)
    cursor = conn.cursor()
    
    with open(csv_file, 'r', encoding='utf-8') as file:
        csv_reader = csv.reader(file)
        
        for row_num, row in enumerate(csv_reader, 1):
            if len(row) < 3:
                print(f"Ошибка в строке {row_num}: неверное количество колонок ({len(row)} вместо 3)")
                continue
            
            subject_name = row[0]
            group_name = row[1]
            professor_username = row[2]
            
            try:
                # Получаем ID предмета
                cursor.execute("SELECT id FROM subjects WHERE name = ?", (subject_name,))
                subject_result = cursor.fetchone()
                
                if not subject_result:
                    print(f"Ошибка в строке {row_num}: предмет '{subject_name}' не найден")
                    continue
                
                subject_id = subject_result[0]
                
                # Получаем ID профессора
                cursor.execute("SELECT id FROM users WHERE username = ?", (professor_username,))
                professor_result = cursor.fetchone()
                
                if not professor_result:
                    print(f"Ошибка в строке {row_num}: профессор '{professor_username}' не найден")
                    continue
                
                professor_id = professor_result[0]
                
                # Получаем ID группы
                cursor.execute("SELECT id FROM groups WHERE name = ?", (group_name,))
                group_result = cursor.fetchone()
                
                if not group_result:
                    print(f"Ошибка в строке {row_num}: группа '{group_name}' не найдена")
                    continue
                
                group_id = group_result[0]
                
                # Создаем курс
                cursor.execute("""
                    INSERT INTO courses (subject_id)
                    VALUES (?)
                """, (subject_id,))
                
                course_id = cursor.lastrowid
                
                # Добавляем профессора как администратора курса
                cursor.execute("""
                    INSERT INTO course_admins (user_id, course_id)
                    VALUES (?, ?)
                """, (professor_id, course_id))
                
                # Добавляем группу как участника курса
                cursor.execute("""
                    INSERT INTO course_attendees (course_id, group_id)
                    VALUES (?, ?)
                """, (course_id, group_id))
                
                print(f"Добавлен курс: {subject_name} для группы {group_name} (преподаватель: {professor_username})")
                
            except Exception as e:
                print(f"Ошибка в строке {row_num}: {e}")
    
    conn.commit()
    conn.close()
    print("Импорт курсов завершен")

def main():
    if len(sys.argv) != 2:
        print("Использование: python import_data.py <база_данных.db>")
        print("Файлы должны называться: users.csv, subjects.csv, courses.csv")
        sys.exit(1)
    
    db_file = sys.argv[1]
    
    # Создаем базу данных
    setup_database(db_file)
    
    # Импортируем данные
    if os.path.exists('users.csv'):
        import_users('users.csv', db_file)
    else:
        print("Файл users.csv не найден")
    
    if os.path.exists('subjects.csv'):
        import_subjects('subjects.csv', db_file)
    else:
        print("Файл subjects.csv не найден")
    
    if os.path.exists('courses.csv'):
        import_courses('courses.csv', db_file)
    else:
        print("Файл courses.csv не найден")
    
    print("\nИмпорт всех данных завершен!")

if __name__ == "__main__":
    main()