import _ from 'lodash';
import { Route, Location } from 'vue-router';
import { Dictionary } from './dict';

export interface RouteArgs {
	dataset?: string,
	terms?: string,
	filters?: string,
	training?: string,
	target?: string,
	results?: string,
	pipelineId?: string,
	residualThreshold?: number
}

/**
 * Builds a route entry object that can be directly pushed onto the stack
 * via  call to route.push(). This holds all the app view state to support
 * nav bar navigation.
 *
 * @param {string} path - route path
 * @param {RouteArgs} args - the arguments for the route.
 */
export function createRouteEntry(path: string, args: RouteArgs = {}): Location {
	const query: Dictionary<string> = {};

	if (args.dataset) { query.dataset = args.dataset; }
	if (args.terms) { query.terms = args.terms; }
	if (args.target) { query.target = args.target; }
	if (args.pipelineId) { query.pipelineId = args.pipelineId; }
	if (!_.isEmpty(args.filters)) { query.filters = args.filters; }
	if (!_.isEmpty(args.training)) { query.training = args.training; }
	if (!_.isEmpty(args.results)) { query.results = args.results; }

	const routeEntry: Location = {
		path: path,
		query: query
	};

	return routeEntry;
}

export function overlayRouteEntry(route: Route, args: RouteArgs): Location {
	// initialize a new object from the supplied route
	const routeEntry: Location = {
		path: route.path,
		query: _.cloneDeep(route.query)
	};

	// merge in the supplied arguments
	_.merge(routeEntry.query, args);

	return routeEntry;
}

export function getRouteFacetPage(key: string, route: Route): number {
	const page = route.query[key];
	return page ? parseInt(page) : 1;
}
