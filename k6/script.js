import http from 'k6/http';
import { check, sleep } from 'k6';

// конфиг нагрузки
export const options = { vus: 5, duration: '20s' };

// базовый URL: можно переопределить переменной BASE_URL
const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const H = { 'Content-Type': 'application/json' };

// подготовка данных: создаём команду (если уже есть — ок)
export function setup() {
  const body = JSON.stringify({
    team_name: 'backend',
    members: [
      { user_id: 'u1', username: 'Alice', is_active: true },
      { user_id: 'u2', username: 'Bob',   is_active: true },
      { user_id: 'u3', username: 'Carol', is_active: true },
      { user_id: 'u4', username: 'Dave',  is_active: true },
    ],
  });
  const res = http.post(`${BASE}/team/add`, body, { headers: H });
  // допустимы два статуса: 201 (создано) или 400 TEAM_EXISTS
  check(res, { 'seed ok': r => r.status === 201 || r.status === 400 });
}

// основной сценарий: create -> merge (идемпотентность не проверяем тут, можно добавить при желании)
export default function () {
  const id = `pr-${__VU}-${Date.now()}`;
  let r = http.post(`${BASE}/pullRequest/create`,
    JSON.stringify({ pull_request_id: id, pull_request_name: 'load', author_id: 'u1' }),
    { headers: H });
  check(r, { 'create=201': res => res.status === 201 });

  r = http.post(`${BASE}/pullRequest/merge`,
    JSON.stringify({ pull_request_id: id }),
    { headers: H });
  check(r, { 'merge=200': res => res.status === 200 });

  sleep(0.2);
}
