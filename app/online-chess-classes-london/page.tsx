import LocationPageTemplate, { getLocationMetadata } from '../components/LocationPageTemplate';
import { Metadata } from 'next';

export const metadata: Metadata = getLocationMetadata('london');

export default function Page() {
  return <LocationPageTemplate cityId="london" />;
}
