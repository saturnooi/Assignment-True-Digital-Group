import http from 'k6/http'
import { check } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080'

export const errorRate = new Rate('error_rate')
export const batchLatency = new Trend('batch_latency')

export const options = {
  scenarios: {
    batch_load: {
      executor: 'constant-arrival-rate',
      rate: 200,      
      timeUnit: '1s',
      duration: '2m',
      preAllocatedVUs: 30,
      maxVUs: 300,
    },
  },

  thresholds: {
    http_req_duration: [
      'avg<300',
      'p(95)<500',
      'p(99)<700',
    ],
    error_rate: ['rate<0.01'],
    batch_latency: ['p(95)<500'],
  },
}

export default function () {
  const res = http.get(
    `${BASE_URL}/recommendations/batch?page=1&limit=20`,
    { tags: { endpoint: 'batch' } }
  )

  batchLatency.add(res.timings.duration)

  const success = check(res, {
    'status 200': (r) => r.status === 200,
  })

  errorRate.add(!success)
}