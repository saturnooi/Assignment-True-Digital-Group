import http from 'k6/http'
import { check } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'

export const errorRate = new Rate('error_rate')
export const cacheLatency = new Trend('cache_latency')

export const options = {
  scenarios: {
    cache_load: {
      executor: 'constant-arrival-rate',
      rate: 200,    
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },

  thresholds: {
    http_req_duration: [
      'avg<150',
      'p(95)<200',
      'p(99)<300',
    ],
    error_rate: ['rate<0.01'],
    cache_latency: ['p(95)<200'],
  },
}

export default function () {
  const res = http.get(
    `${BASE_URL}/users/1/recommendations?limit=10`,
    { tags: { endpoint: 'cache' } }
  )

  cacheLatency.add(res.timings.duration)

  const success = check(res, {
    'status 200': (r) => r.status === 200,
  })

  errorRate.add(!success)
}