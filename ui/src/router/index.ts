import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'ExperimentList',
      component: () => import('@/views/ExperimentList.vue')
    },
    {
      path: '/experiment/:id',
      name: 'ExperimentDetail',
      component: () => import('@/views/ExperimentDetail.vue'),
      props: true
    },
    {
      path: '/verify',
      name: 'OnlineVerify',
      component: () => import('@/views/OnlineVerify.vue')
    }
  ]
})

export default router
