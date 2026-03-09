import http from 'k6/http'
import { check } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'
const MAX_USER = Number(__ENV.MAX_USER || 2000)

export const errorRate = new Rate('error_rate')
export const singleLatency = new Trend('single_latency')

export const options = {
  scenarios: {
    single_load: {
      executor: 'constant-arrival-rate',
      rate: 200,     
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 50,
      maxVUs: 300,
    },
  },

  thresholds: {
    http_req_duration: [
      'avg<200',
      'p(95)<300',
      'p(99)<500',
    ],
    error_rate: ['rate<0.01'],
    single_latency: ['p(95)<300'],
  },
}

export default function () {
  const userId = Math.floor(Math.random() * MAX_USER) + 1

  const res = http.get(
    `${BASE_URL}/users/${userId}/recommendations?limit=10`,
    { tags: { endpoint: 'single' } }
  )

  singleLatency.add(res.timings.duration)

  const success = check(res, {
    'status 200': (r) => r.status === 200,
    'has recommendations': (r) => {
      try { return JSON.parse(r.body).recommendations.length > 0 } catch { return false }
    },
  })

  errorRate.add(!success)
}