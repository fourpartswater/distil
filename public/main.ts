import Vue from 'vue';
import VueRouter from 'vue-router';
import VueRouterSync from 'vuex-router-sync';
import Home from './views/Home.vue';
import Search from './views/Search.vue';
import SelectTarget from './views/SelectTarget.vue';
import SelectTraining from './views/SelectTraining.vue';
import Results from './views/Results.vue';
import Navigation from './views/Navigation.vue';
import ExportSuccess from './views/ExportSuccess.vue';
import AbortSuccess from './views/AbortSuccess.vue';
import { getters as routeGetters } from './store/route/module';
import { mutations as viewMutations } from './store/view/module';
import { getters as appGetters, actions as appActions } from './store/app/module';
import { ROOT_ROUTE, HOME_ROUTE, SEARCH_ROUTE, SELECT_ROUTE, CREATE_ROUTE, RESULTS_ROUTE, EXPORT_SUCCESS_ROUTE, ABORT_SUCCESS_ROUTE } from './store/route/index';
import store from './store/store';
import BootstrapVue from 'bootstrap-vue';

import 'bootstrap-vue/dist/bootstrap-vue.css';

import './styles/bootstrap-v4beta2-custom.css';
import './styles/main.css';

Vue.use(VueRouter);
Vue.use(BootstrapVue);

export const router = new VueRouter({
	routes: [
		{ path: ROOT_ROUTE, redirect: HOME_ROUTE },
		{ path: HOME_ROUTE, component: Home },
		{ path: SEARCH_ROUTE, component: Search },
		{ path: SELECT_ROUTE, component: SelectTarget },
		{ path: CREATE_ROUTE, component: SelectTraining },
		{ path: RESULTS_ROUTE, component: Results },
		{ path: EXPORT_SUCCESS_ROUTE, component: ExportSuccess },
		{ path: ABORT_SUCCESS_ROUTE, component: AbortSuccess }
	]
});

router.beforeEach((route, _, next) => {
	const dataset = route.query ? route.query.dataset : routeGetters.getRouteDataset(store);
	viewMutations.saveView(store, {
		view: route.path,
		dataset: dataset,
		route: route
	});
	next();
});

// sync store and router
VueRouterSync.sync(store, router, { moduleName: 'routeModule' });

// init app
new Vue({
	store,
	router,
	components: {
		Navigation
	},
	template: `
		<div id="distil-app">
			<navigation></navigation>
			<router-view class="view"></router-view>
		</div>`,
	beforeMount() {
		appActions.fetchConfig(this.$store);
		if (appGetters.isDiscovery(this.$store)) {
			console.log('shieeet');
		}
	}
}).$mount('#app');
