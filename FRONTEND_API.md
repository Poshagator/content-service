# Frontend API: Educational Modules

This documentation describes how to interact with the educational and timetable endpoints of the `content-service`.

**Base URL:** `https://api.poshagator.ru/content/edu`

---

## 1. Groups (Группы)

### List Groups
Retrieve a paginated list of educational groups for a specific filial.

**Endpoint:** `GET /group/list`

**Parameters:**
- `filialID` (string, UUID): The unique identifier of the filial.
- `page` (int): Page number (starting from 1).
- `perPage` (int): Number of items per page.

**Example Request:**
```bash
curl "https://api.poshagator.ru/content/edu/group/list?filialID=4888f1e4-5916-45ef-a485-0d0381872968&page=1&perPage=20"
```

**Example Response:**
```json
[
  {
    "id": "ac9e6515-0386-40eb-8a12-175fee6b5b2c",
    "filial_id": "4888f1e4-5916-45ef-a485-0d0381872968",
    "name": "1-02_65274"
  },
  {
    "id": "46c1f5a3-5f45-4ed4-89e1-ad50998e9a1d",
    "filial_id": "4888f1e4-5916-45ef-a485-0d0381872968",
    "name": "1-01_65274"
  }
]
```

### Search Groups
Search for educational groups by name (partial match, case-insensitive).

**Endpoint:** `GET /group/search`

**Parameters:**
- `filialID` (string, UUID): The unique identifier of the filial.
- `name` (string): Part of the group name to search for.
- `limit` (int, optional): Max results to return (default: 20).

**Example Request:**
```bash
curl "https://api.poshagator.ru/content/edu/group/search?filialID=4888f1e4-5916-45ef-a485-0d0381872968&name=1МО"
```

---

## 2. Timetable (Расписание)

### Get Group Schedule
Retrieve the full timetable for a specific group.

**Endpoint:** `GET /timetable/list`

**Parameters:**
- `groupID` (string, UUID): The unique identifier of the group.

**Example Request:**
```bash
curl "https://api.poshagator.ru/content/edu/timetable/list?groupID=a128672d-e866-45e1-94e8-0ad5e0beb0f5"
```

**Example Response:**
```json
[
  {
    "id": "5f3a1e2b-3c4d-5e6f-7g8h-9i0j1k2l3m4n",
    "day_of_week": 1,
    "starts_at": "09:00:00",
    "ends_at": "10:20:00",
    "week_type": "ALL",
    "subject_name": "Macroeconomics",
    "teacher_name": "Ivanov Ivan Ivanovich",
    "room_name": "2014"
  },
  {
    "id": "a1b2c3d4-e5f6-g7h8-i9j0-k1l2m3n4o5p6",
    "day_of_week": 1,
    "starts_at": "10:35:00",
    "ends_at": "11:55:00",
    "week_type": "ALL",
    "subject_name": "History of Russia",
    "teacher_name": "Petrov Petr Petrovich",
    "room_name": "1076"
  }
]
```

**Response Fields:**
- `day_of_week`: 1 (Monday) to 7 (Sunday).
- `starts_at` / `ends_at`: Time in `HH:MM:SS` format.
- `week_type`: `ALL` (default), `ODD` (нечетная), or `EVEN` (четная).
- `subject_name`: The title of the lesson/subject.
- `teacher_name`: Full name of the teacher.
- `room_name`: Room number or title.

---

## 3. Academic Terms (Семестры)

### List Terms
Retrieve a list of academic terms for a filial.

**Endpoint:** `GET /term/list`

**Example Request:**
```bash
curl "https://api.poshagator.ru/content/edu/term/list?filialID=4888f1e4-5916-45ef-a485-0d0381872968&page=1&perPage=10"
```
