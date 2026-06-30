import LocationPageTemplate, { getLocationMetadata } from '../components/LocationPageTemplate';
import { Metadata } from 'next';

export const metadata: Metadata = getLocationMetadata('bengaluru');

export default function Page() {
  return <LocationPageTemplate cityId="bengaluru" />;
}
