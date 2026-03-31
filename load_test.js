import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 50 }, // Ramp up to 50 concurrent virtual users
        { duration: '1m', target: 50 },  // Hold at 50 users
        { duration: '30s', target: 0 },  // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<1000'], // 95% of requests should be below 1s (ideal)
        http_req_failed: ['rate<0.01'],    // Error rate should be less than 1%
    },
};

const BASE_URL = 'http://localhost:8082';

export function setup() {
    // Try hitting the login page to ensure server is up
    console.log("Setting up k6 test against " + BASE_URL);
}

export default function () {
    // 1. Authenticate the Virtual User
    // Note: Cookies are automatically managed per-VU by k6 
    const loginRes = http.post(`${BASE_URL}/login`, {
        password: 'bluesmusic'
    });
    
    check(loginRes, {
        'login successful': (r) => r.status === 200 || r.status === 303 || r.status === 302,
    });

    // 2. Fire the generation request with mock payload
    const payload = {
        prompt: 'A pristine clean tone for modern worship',
        mock: 'true'
    };

    const start = new Date();
    const generateRes = http.post(`${BASE_URL}/api/preset/generate`, payload, {
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
        }
    });

    check(generateRes, {
        'generate status is 200': (r) => r.status === 200,
        'response contains draft preset html': (r) => r.body.includes('Draft Preset'),
    });

    // 3. Keep some spacing between generations to prevent unnatural continuous hammering
    sleep(1);
}
