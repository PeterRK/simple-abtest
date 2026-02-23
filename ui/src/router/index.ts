import { createRouter, createWebHistory } from 'vue-router'
import ExperimentList from '@/views/ExperimentList.vue'
import ExperimentDetail from '@/views/ExperimentDetail.vue'
import OnlineVerify from '@/views/OnlineVerify.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'ExperimentList',
      component: ExperimentList
    },
    {
      path: '/experiment/:id',
      name: 'ExperimentDetail',
      component: ExperimentDetail,
      props: true
    },
    {
      path: '/verify',
      name: 'OnlineVerify',
      component: OnlineVerify
    }
  ]
})

export default router
