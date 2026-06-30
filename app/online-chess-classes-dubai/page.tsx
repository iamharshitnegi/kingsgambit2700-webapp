import LocationPageTemplate, { getLocationMetadata } from '../components/LocationPageTemplate';
import { Metadata } from 'next';

export const metadata: Metadata = getLocationMetadata('dubai');

export default function Page() {
  return <LocationPageTemplate cityId="dubai" />;
}
